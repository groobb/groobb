package database_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/groobb/groobb/go/internal/database"
)

// openTestDBは使い捨てのファイル上にSQLiteデータベースを開き、パスとともに
// 返します。テスト終了時にクローズします。インメモリではなくファイルを使うのは、
// 設定対象であるWALとロックの挙動をプールに実際に通すためです。
func openTestDB(t *testing.T) (*database.DB, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "groobb.sqlite")

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("データベースのオープンに失敗: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("データベースのクローズに失敗: %v", err)
		}
	})

	return db, path
}

// TestOpen_AppliesPragmasは、両プールが遅延して開くすべてのコネクションに
// PRAGMAが適用済みであることを検証します。複数の読み取りコネクションを同時に
// 保持することで、起動時のpingで作られたもの以外もdatabase/sqlに開かせます。
// Openの後に1度だけ文を実行する方式では、このコネクションを設定できません。
func TestOpen_AppliesPragmas(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	pools := []struct {
		name          string
		pool          *sql.DB
		connections   int
		wantQueryOnly string
	}{
		{name: "writer", pool: db.Writer, connections: 1, wantQueryOnly: "0"},
		{name: "reader", pool: db.Reader, connections: 2, wantQueryOnly: "1"},
	}

	pragmas := []struct {
		pragma string
		want   string
	}{
		{pragma: "journal_mode", want: "wal"},
		{pragma: "foreign_keys", want: "1"},
		{pragma: "synchronous", want: "1"},
		{pragma: "busy_timeout", want: "5000"},
	}

	for _, pool := range pools {
		t.Run(pool.name, func(t *testing.T) {
			t.Parallel()

			// PRAGMAの設定はコネクションごとです。検証前にすべてを取得して
			// 保持することで、2本目のReaderが最初のものの再利用ではなく、遅延して
			// 開かれた別のコネクションであることを保証します。
			connections := make([]*sql.Conn, 0, pool.connections)
			for range pool.connections {
				conn, err := pool.pool.Conn(context.Background())
				if err != nil {
					t.Fatalf("コネクションの取得に失敗: %v", err)
				}
				connections = append(connections, conn)
			}
			t.Cleanup(func() {
				for _, conn := range connections {
					_ = conn.Close()
				}
			})

			for i, conn := range connections {
				for _, p := range pragmas {
					var got string
					if err := conn.QueryRowContext(context.Background(), "PRAGMA "+p.pragma).Scan(&got); err != nil {
						t.Fatalf("コネクション %d: PRAGMA %s の読み取りに失敗: %v", i, p.pragma, err)
					}
					if got != p.want {
						t.Errorf("コネクション %d: PRAGMA %s = %q、期待値 = %q", i, p.pragma, got, p.want)
					}
				}

				var gotQueryOnly string
				if err := conn.QueryRowContext(context.Background(), "PRAGMA query_only").Scan(&gotQueryOnly); err != nil {
					t.Fatalf("コネクション %d: PRAGMA query_onlyの読み取りに失敗: %v", i, err)
				}
				if gotQueryOnly != pool.wantQueryOnly {
					t.Errorf("コネクション %d: PRAGMA query_only = %q、期待値 = %q", i, gotQueryOnly, pool.wantQueryOnly)
				}
			}
		})
	}
}

// TestOpen_PoolSizesは、書き込みが1本のコネクションに直列化され、
// 読み取りプールが設定どおりのCPUベースの本数であることを検証します。
func TestOpen_PoolSizes(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if got := db.Writer.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("書き込みプールの最大コネクション数 = %d、期待値 = 1", got)
	}

	wantReaderConns := max(runtime.NumCPU(), 4)
	if got := db.Reader.Stats().MaxOpenConnections; got != wantReaderConns {
		t.Errorf("読み取りプールの最大コネクション数 = %d、期待値 = %d", got, wantReaderConns)
	}
}

// TestOpen_PreservesSpecialCharactersInPathは、ファイルシステムのパスに含まれる
// URIの区切り文字が、DSNのクエリやフラグメントではなくファイル名の一部として
// エンコードされることを検証します。
func TestOpen_PreservesSpecialCharactersInPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "groobb?archive%#sqlite")
	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("データベースのオープンに失敗: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("データベースのクローズに失敗: %v", err)
		}
	})

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("テーブルの作成に失敗: %v", err)
	}
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
		t.Fatalf("行の挿入に失敗: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("行の読み戻しに失敗: %v", err)
	}
	if name != "groobb" {
		t.Errorf("読み戻した値 = %q、期待値 = %q", name, "groobb")
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("指定したパスにデータベースが作られていない: %v", err)
	}
}

// TestOpen_TreatsSQLiteSpecialNamesAsFilePathsは、SQLiteが特別扱いする相対名でも
// 通常のファイルが作られ、両プールが同じファイルへ接続することを検証します。
//
// これらの名前は相対パスとしてしか意味を持たないため、本テストはt.Chdirで一時
// ディレクトリへ移動します。本テストもサブテストもt.Parallel() を呼ばないのはこのため
// です。作業ディレクトリは1つのテストではなくプロセスに属するので、t.Chdirは並列
// テストや並列な祖先を持つテストでpanicします。
func TestOpen_TreatsSQLiteSpecialNamesAsFilePaths(t *testing.T) {
	for _, name := range []string{":memory:", "file::memory:"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			db, err := database.Open(context.Background(), name)
			if err != nil {
				t.Fatalf("データベースのオープンに失敗: %v", err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Errorf("データベースのクローズに失敗: %v", err)
				}
			})

			if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
				t.Fatalf("テーブルの作成に失敗: %v", err)
			}
			if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
				t.Fatalf("行の挿入に失敗: %v", err)
			}

			var got string
			if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&got); err != nil {
				t.Fatalf("行の読み戻しに失敗: %v", err)
			}
			if got != "groobb" {
				t.Errorf("読み戻した値 = %q、期待値 = %q", got, "groobb")
			}

			if _, err := os.Stat(name); err != nil {
				t.Errorf("データベースが %q という名前の通常のファイルとして作られていない: %v", name, err)
			}
		})
	}
}

// TestOpen_AcceptsPathWithLeadingDoubleSlashは、スラッシュ2つで始まるパスが、
// その名前のファイルを開くことを検証します。空のオーソリティを前に置かない限り、この
// 2つのスラッシュは "file:" URIのオーソリティの開始になり、SQLiteは空と "localhost"
// 以外のオーソリティをすべて拒否します。
func TestOpen_AcceptsPathWithLeadingDoubleSlash(t *testing.T) {
	t.Parallel()

	// Linuxでは先頭の "//" はスラッシュ1つと同じファイルを指すため、
	// データベースはテストの一時ディレクトリに作られる。
	path := "/" + filepath.Join(t.TempDir(), "groobb.sqlite")

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("データベースのオープンに失敗: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("データベースのクローズに失敗: %v", err)
		}
	})

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("テーブルの作成に失敗: %v", err)
	}
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
		t.Fatalf("行の挿入に失敗: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("行の読み戻しに失敗: %v", err)
	}
	if name != "groobb" {
		t.Errorf("読み戻した値 = %q、期待値 = %q", name, "groobb")
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("指定したパスにデータベースが作られていない: %v", err)
	}
}

// TestOpen_EnforcesForeignKeysは外部キー制約が実際に効くことを検証します。
// SQLiteはコネクションごとに有効化しない限り外部キーを無視するため、PRAGMAが
// 抜けていると孤児レコードがエラーなく書き込めてしまいます。
func TestOpen_EnforcesForeignKeys(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if _, err := db.Writer.ExecContext(context.Background(), `
		CREATE TABLE parents (id INTEGER PRIMARY KEY);
		CREATE TABLE children (
			id INTEGER PRIMARY KEY,
			parent_id INTEGER NOT NULL REFERENCES parents(id)
		);
	`); err != nil {
		t.Fatalf("テーブルの作成に失敗: %v", err)
	}

	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO children (parent_id) VALUES (1)"); err == nil {
		t.Fatal("外部キーに違反する行の挿入でエラーを期待したが、nilだった")
	}
}

// TestOpen_WriteTransactionTakesTheLockImmediatelyは、書き込みプールが
// トランザクションをBEGIN IMMEDIATEで開始することを検証します。deferredなBEGINは
// 最初の書き込みまでロックを取らず、SQLiteは読み取りロックからの昇格にbusy_timeoutを
// 適用しないため、deferredな書き込みトランザクションは他のライターを待たずに
// SQLITE_BUSYで失敗します。
func TestOpen_WriteTransactionTakesTheLockImmediately(t *testing.T) {
	t.Parallel()

	db, path := openTestDB(t)

	// 同じファイルに対する別のプールで他プロセスを模す。busy_timeoutを0に
	// するのは、アプリケーションのプールが使うタイムアウトを待たずに検証するため。
	//
	// パスを "file:" URIに入れるのは接続層と同じ理由で、素のパスにクエリを連結すると、
	// ファイル名に含まれる区切り文字のところでドライバがSQLiteへ渡す名前が切れる。
	otherParams := url.Values{}
	otherParams.Set("_txlock", "immediate")
	otherParams.Set("_busy_timeout", "0")
	otherDSN := url.URL{Scheme: "file", Path: path, RawQuery: otherParams.Encode()}

	other, err := sql.Open("sqlite", otherDSN.String())
	if err != nil {
		t.Fatalf("2つ目のプールのオープンに失敗: %v", err)
	}
	defer func() { _ = other.Close() }()

	tx, err := db.Writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("書き込みトランザクションの開始に失敗: %v", err)
	}

	if otherTx, err := other.BeginTx(context.Background(), nil); err == nil {
		_ = otherTx.Rollback()
		t.Error("書き込みトランザクションの実行中に2つ目のライターがトランザクションを開始できた。書き込みプールがBEGINでロックを取っていない")
	}

	// 読み取りプールは書き込みロックを取ってはならない。書き込みトランザクションの
	// 実行中も読み取りは動き続ける必要がある。
	readTx, err := db.Reader.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("書き込みトランザクションの実行中に読み取りトランザクションの開始に失敗: %v", err)
	}
	if err := readTx.Rollback(); err != nil {
		t.Fatalf("読み取りトランザクションのロールバックに失敗: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("書き込みトランザクションのロールバックに失敗: %v", err)
	}

	otherTx, err := other.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("書き込みトランザクションの終了後にトランザクションの開始に失敗: %v", err)
	}
	if err := otherTx.Rollback(); err != nil {
		t.Fatalf("2つ目のトランザクションのロールバックに失敗: %v", err)
	}
}

// TestOpen_ReaderSeesCommittedWritesは、2つのプールが同じデータベース
// ファイルに紐づいており、コミット済みの書き込みが読み取り側から見えることを
// 検証します。
func TestOpen_ReaderSeesCommittedWrites(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("テーブルの作成に失敗: %v", err)
	}
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
		t.Fatalf("行の挿入に失敗: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("行の読み戻しに失敗: %v", err)
	}
	if name != "groobb" {
		t.Errorf("読み戻した値 = %q、期待値 = %q", name, "groobb")
	}
}

// TestOpen_ReaderRejectsWritesは、読み取りプールが読み取りを処理しつつ、
// コネクションレベルで書き込みを拒否することを検証します。
func TestOpen_ReaderRejectsWrites(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("テーブルの作成に失敗: %v", err)
	}
	if _, err := db.Reader.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('reader')"); err == nil {
		t.Fatal("読み取りプールが書き込みを拒否することを期待したが、nilだった")
	}

	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('writer')"); err != nil {
		t.Fatalf("書き込みプールを通した挿入に失敗: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("読み取りプールを通した読み取りに失敗: %v", err)
	}
	if name != "writer" {
		t.Errorf("読み戻した値 = %q、期待値 = %q", name, "writer")
	}
}

// TestOpen_EmptyPathは、データベースのパスが未設定のときに拒否されることを
// 検証します。そうしないとSQLiteはprivateな一時データベースを開き、書き込みを
// 受け付けたうえでクローズ時に捨ててしまいます。
func TestOpen_EmptyPath(t *testing.T) {
	t.Parallel()

	db, err := database.Open(context.Background(), "")
	if err == nil {
		_ = db.Close()
		t.Fatal("空のデータベースファイルのパスでエラーを期待したが、nilだった")
	}
}

// TestOpen_UnopenablePathは、プロセスが開けないパスが最初のリクエストではなく
// 起動時に失敗することを検証します。
func TestOpen_UnopenablePath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "groobb.sqlite")

	db, err := database.Open(context.Background(), path)
	if err == nil {
		_ = db.Close()
		t.Fatal("存在しないディレクトリの下のデータベースファイルでエラーを期待したが、nilだった")
	}
}

// TestDB_CloseはCloseが両方のプールを停止させることを検証します。
func TestDB_Close(t *testing.T) {
	t.Parallel()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "groobb.sqlite"))
	if err != nil {
		t.Fatalf("データベースのオープンに失敗: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("データベースのクローズに失敗: %v", err)
	}

	for name, pool := range map[string]*sql.DB{"writer": db.Writer, "reader": db.Reader} {
		if err := pool.PingContext(context.Background()); err == nil {
			t.Errorf("Close後も %s プールが使える", name)
		}
	}
}
