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

// openTestDB opens a SQLite database on a throwaway file and returns it with
// the path, closing it when the test ends. A file is used rather than an
// in-memory database so the pools exercise the WAL and locking behaviour they
// are configured for.
//
// [Ja] openTestDB は使い捨てのファイル上に SQLite データベースを開き、パスとともに
// 返します。テスト終了時にクローズします。インメモリではなくファイルを使うのは、
// 設定対象である WAL とロックの挙動をプールに実際に通すためです。
func openTestDB(t *testing.T) (*database.DB, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "groobb.sqlite")

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return db, path
}

// TestOpen_AppliesPragmas verifies that every connection opened lazily by both
// pools has its PRAGMAs already applied. Holding multiple reader connections at
// once forces database/sql to open beyond the connection created by the startup
// ping, which a one-off statement after Open would not configure.
//
// [Ja] TestOpen_AppliesPragmas は、両プールが遅延して開くすべてのコネクションに
// PRAGMA が適用済みであることを検証します。複数の読み取りコネクションを同時に
// 保持することで、起動時の ping で作られたもの以外も database/sql に開かせます。
// Open の後に 1 度だけ文を実行する方式では、このコネクションを設定できません。
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

			// PRAGMA settings are per connection. Take and hold every connection
			// before asserting so the second reader is guaranteed to be a distinct,
			// lazily opened connection rather than the pool reusing the first one.
			//
			// [Ja] PRAGMA の設定はコネクションごとです。検証前にすべてを取得して
			// 保持することで、2 本目の Reader が最初のものの再利用ではなく、遅延して
			// 開かれた別のコネクションであることを保証します。
			connections := make([]*sql.Conn, 0, pool.connections)
			for range pool.connections {
				conn, err := pool.pool.Conn(context.Background())
				if err != nil {
					t.Fatalf("failed to take a connection: %v", err)
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
						t.Fatalf("connection %d: failed to read PRAGMA %s: %v", i, p.pragma, err)
					}
					if got != p.want {
						t.Errorf("connection %d: PRAGMA %s = %q, want %q", i, p.pragma, got, p.want)
					}
				}

				var gotQueryOnly string
				if err := conn.QueryRowContext(context.Background(), "PRAGMA query_only").Scan(&gotQueryOnly); err != nil {
					t.Fatalf("connection %d: failed to read PRAGMA query_only: %v", i, err)
				}
				if gotQueryOnly != pool.wantQueryOnly {
					t.Errorf("connection %d: PRAGMA query_only = %q, want %q", i, gotQueryOnly, pool.wantQueryOnly)
				}
			}
		})
	}
}

// TestOpen_PoolSizes verifies that writes are serialized onto a single
// connection while the read pool has exactly the configured CPU-based size.
//
// [Ja] TestOpen_PoolSizes は、書き込みが 1 本のコネクションに直列化され、
// 読み取りプールが設定どおりの CPU ベースの本数であることを検証します。
func TestOpen_PoolSizes(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if got := db.Writer.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("the write pool allows %d connections, want 1", got)
	}

	wantReaderConns := max(runtime.NumCPU(), 4)
	if got := db.Reader.Stats().MaxOpenConnections; got != wantReaderConns {
		t.Errorf("the read pool allows %d connections, want %d", got, wantReaderConns)
	}
}

// TestOpen_PreservesSpecialCharactersInPath verifies that URI delimiters in a
// filesystem path are encoded as part of the filename instead of being parsed
// as the DSN query or fragment.
//
// [Ja] TestOpen_PreservesSpecialCharactersInPath は、ファイルシステムのパスに含まれる
// URI の区切り文字が、DSN のクエリやフラグメントではなくファイル名の一部として
// エンコードされることを検証します。
func TestOpen_PreservesSpecialCharactersInPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "groobb?archive%#sqlite")
	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create the table: %v", err)
	}
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
		t.Fatalf("failed to insert the row: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("failed to read the row back: %v", err)
	}
	if name != "groobb" {
		t.Errorf("read back %q, want %q", name, "groobb")
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("the database was not created at the requested path: %v", err)
	}
}

// TestOpen_TreatsSQLiteSpecialNamesAsFilePaths verifies that relative names
// recognized specially by SQLite still create ordinary files and that both
// pools connect to the same file.
//
// The names only mean anything as relative paths, so the test moves into a
// temporary directory with t.Chdir. That is why neither this test nor its
// subtests call t.Parallel(): the working directory belongs to the process
// rather than to one test, so t.Chdir panics in a parallel test or in one with
// a parallel ancestor.
//
// [Ja] TestOpen_TreatsSQLiteSpecialNamesAsFilePaths は、SQLite が特別扱いする相対名でも
// 通常のファイルが作られ、両プールが同じファイルへ接続することを検証します。
//
// これらの名前は相対パスとしてしか意味を持たないため、本テストは t.Chdir で一時
// ディレクトリへ移動します。本テストもサブテストも t.Parallel() を呼ばないのはこのため
// です。作業ディレクトリは 1 つのテストではなくプロセスに属するので、t.Chdir は並列
// テストや並列な祖先を持つテストで panic します。
func TestOpen_TreatsSQLiteSpecialNamesAsFilePaths(t *testing.T) {
	for _, name := range []string{":memory:", "file::memory:"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			db, err := database.Open(context.Background(), name)
			if err != nil {
				t.Fatalf("failed to open the database: %v", err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Errorf("failed to close the database: %v", err)
				}
			})

			if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
				t.Fatalf("failed to create the table: %v", err)
			}
			if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
				t.Fatalf("failed to insert the row: %v", err)
			}

			var got string
			if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&got); err != nil {
				t.Fatalf("failed to read the row back: %v", err)
			}
			if got != "groobb" {
				t.Errorf("read back %q, want %q", got, "groobb")
			}

			if _, err := os.Stat(name); err != nil {
				t.Errorf("the database was not created as an ordinary file named %q: %v", name, err)
			}
		})
	}
}

// TestOpen_AcceptsPathWithLeadingDoubleSlash verifies that a path beginning
// with two slashes opens the file it names. Those two slashes start the
// authority of a "file:" URI unless an empty one is written before them, and
// SQLite rejects every authority other than an empty one or "localhost".
//
// [Ja] TestOpen_AcceptsPathWithLeadingDoubleSlash は、スラッシュ 2 つで始まるパスが、
// その名前のファイルを開くことを検証します。空のオーソリティを前に置かない限り、この
// 2 つのスラッシュは "file:" URI のオーソリティの開始になり、SQLite は空と "localhost"
// 以外のオーソリティをすべて拒否します。
func TestOpen_AcceptsPathWithLeadingDoubleSlash(t *testing.T) {
	t.Parallel()

	// A leading "//" names the same file as a single slash on Linux, so the
	// database still lands in the test's temporary directory.
	//
	// [Ja] Linux では先頭の "//" はスラッシュ 1 つと同じファイルを指すため、
	// データベースはテストの一時ディレクトリに作られる。
	path := "/" + filepath.Join(t.TempDir(), "groobb.sqlite")

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create the table: %v", err)
	}
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
		t.Fatalf("failed to insert the row: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("failed to read the row back: %v", err)
	}
	if name != "groobb" {
		t.Errorf("read back %q, want %q", name, "groobb")
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("the database was not created at the requested path: %v", err)
	}
}

// TestOpen_EnforcesForeignKeys verifies that foreign key constraints are
// enforced. SQLite ignores them unless they are switched on per connection, so
// a missing PRAGMA would let orphaned rows be written without any error.
//
// [Ja] TestOpen_EnforcesForeignKeys は外部キー制約が実際に効くことを検証します。
// SQLite はコネクションごとに有効化しない限り外部キーを無視するため、PRAGMA が
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
		t.Fatalf("failed to create the tables: %v", err)
	}

	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO children (parent_id) VALUES (1)"); err == nil {
		t.Fatal("expected an error when inserting a row that violates a foreign key, got nil")
	}
}

// TestOpen_WriteTransactionTakesTheLockImmediately verifies that the write pool
// begins its transactions with BEGIN IMMEDIATE. A deferred BEGIN takes no lock
// until the first write, and SQLite does not apply busy_timeout to the upgrade
// from a read lock, so a deferred write transaction fails with SQLITE_BUSY
// instead of waiting for the other writer.
//
// [Ja] TestOpen_WriteTransactionTakesTheLockImmediately は、書き込みプールが
// トランザクションを BEGIN IMMEDIATE で開始することを検証します。deferred な BEGIN は
// 最初の書き込みまでロックを取らず、SQLite は読み取りロックからの昇格に busy_timeout を
// 適用しないため、deferred な書き込みトランザクションは他のライターを待たずに
// SQLITE_BUSY で失敗します。
func TestOpen_WriteTransactionTakesTheLockImmediately(t *testing.T) {
	t.Parallel()

	db, path := openTestDB(t)

	// A separate pool on the same file stands in for another process. Its
	// busy_timeout is zero so the assertion does not have to wait out the
	// timeout that the application's pools use.
	//
	// The path goes into a "file:" URI for the same reason the connection layer
	// does it: appending the query to a bare path lets a delimiter in the
	// filename truncate the name the driver passes to SQLite.
	//
	// [Ja] 同じファイルに対する別のプールで他プロセスを模す。busy_timeout を 0 に
	// するのは、アプリケーションのプールが使うタイムアウトを待たずに検証するため。
	//
	// パスを "file:" URI に入れるのは接続層と同じ理由で、素のパスにクエリを連結すると、
	// ファイル名に含まれる区切り文字のところでドライバが SQLite へ渡す名前が切れる。
	otherParams := url.Values{}
	otherParams.Set("_txlock", "immediate")
	otherParams.Set("_busy_timeout", "0")
	otherDSN := url.URL{Scheme: "file", Path: path, RawQuery: otherParams.Encode()}

	other, err := sql.Open("sqlite", otherDSN.String())
	if err != nil {
		t.Fatalf("failed to open the second pool: %v", err)
	}
	defer func() { _ = other.Close() }()

	tx, err := db.Writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to begin the write transaction: %v", err)
	}

	if otherTx, err := other.BeginTx(context.Background(), nil); err == nil {
		_ = otherTx.Rollback()
		t.Error("the second writer began a transaction while the write transaction was open, so the write pool is not taking the lock at BEGIN")
	}

	// The read pool must not take the write lock: reads have to keep working
	// while a write transaction is in flight.
	//
	// [Ja] 読み取りプールは書き込みロックを取ってはならない。書き込みトランザクションの
	// 実行中も読み取りは動き続ける必要がある。
	readTx, err := db.Reader.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to begin a read transaction while the write transaction was open: %v", err)
	}
	if err := readTx.Rollback(); err != nil {
		t.Fatalf("failed to roll back the read transaction: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("failed to roll back the write transaction: %v", err)
	}

	otherTx, err := other.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to begin a transaction after the write transaction ended: %v", err)
	}
	if err := otherTx.Rollback(); err != nil {
		t.Fatalf("failed to roll back the second transaction: %v", err)
	}
}

// TestOpen_ReaderSeesCommittedWrites verifies that the two pools are backed by
// the same database file and that a committed write is visible to the reader.
//
// [Ja] TestOpen_ReaderSeesCommittedWrites は、2 つのプールが同じデータベース
// ファイルに紐づいており、コミット済みの書き込みが読み取り側から見えることを
// 検証します。
func TestOpen_ReaderSeesCommittedWrites(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create the table: %v", err)
	}
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('groobb')"); err != nil {
		t.Fatalf("failed to insert the row: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("failed to read the row back: %v", err)
	}
	if name != "groobb" {
		t.Errorf("read back %q, want %q", name, "groobb")
	}
}

// TestOpen_ReaderRejectsWrites verifies that the read pool enforces its
// contract at the connection level while still serving reads.
//
// [Ja] TestOpen_ReaderRejectsWrites は、読み取りプールが読み取りを処理しつつ、
// コネクションレベルで書き込みを拒否することを検証します。
func TestOpen_ReaderRejectsWrites(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if _, err := db.Writer.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create the table: %v", err)
	}
	if _, err := db.Reader.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('reader')"); err == nil {
		t.Fatal("expected the read pool to reject a write, got nil")
	}

	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO items (name) VALUES ('writer')"); err != nil {
		t.Fatalf("failed to insert through the write pool: %v", err)
	}

	var name string
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT name FROM items").Scan(&name); err != nil {
		t.Fatalf("failed to read through the read pool: %v", err)
	}
	if name != "writer" {
		t.Errorf("read back %q, want %q", name, "writer")
	}
}

// TestOpen_EmptyPath verifies that an unset database path is rejected. SQLite
// would otherwise open a private temporary database, which accepts writes and
// discards them on close.
//
// [Ja] TestOpen_EmptyPath は、データベースのパスが未設定のときに拒否されることを
// 検証します。そうしないと SQLite は private な一時データベースを開き、書き込みを
// 受け付けたうえでクローズ時に捨ててしまいます。
func TestOpen_EmptyPath(t *testing.T) {
	t.Parallel()

	db, err := database.Open(context.Background(), "")
	if err == nil {
		_ = db.Close()
		t.Fatal("expected an error for an empty database file path, got nil")
	}
}

// TestOpen_UnopenablePath verifies that a path the process cannot open fails at
// startup rather than on the first request.
//
// [Ja] TestOpen_UnopenablePath は、プロセスが開けないパスが最初のリクエストではなく
// 起動時に失敗することを検証します。
func TestOpen_UnopenablePath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "groobb.sqlite")

	db, err := database.Open(context.Background(), path)
	if err == nil {
		_ = db.Close()
		t.Fatal("expected an error for a database file under a missing directory, got nil")
	}
}

// TestDB_Close verifies that Close shuts down both pools.
//
// [Ja] TestDB_Close は Close が両方のプールを停止させることを検証します。
func TestDB_Close(t *testing.T) {
	t.Parallel()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "groobb.sqlite"))
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close the database: %v", err)
	}

	for name, pool := range map[string]*sql.DB{"writer": db.Writer, "reader": db.Reader} {
		if err := pool.PingContext(context.Background()); err == nil {
			t.Errorf("the %s pool is still usable after Close", name)
		}
	}
}
