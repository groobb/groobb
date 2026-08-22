// Package testutil provides shared helpers for tests, such as a per-test SQLite
// database and the builders that seed rows into it.
//
// [Ja] testutil パッケージは、テスト用の共有ヘルパー (テストごとの SQLite データベースと、
// そこへ行を投入するビルダーなど) を提供します。
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
)

// databaseFileName is the name given to every test database file. Each one
// lives in a directory of its own, so the name only has to be recognizable in
// a stack trace or a leftover temporary directory.
//
// [Ja] databaseFileName はすべてのテスト用データベースファイルに付ける名前です。
// どのファイルもそれぞれ専用のディレクトリに置かれるため、名前に求められるのは
// スタックトレースや残った一時ディレクトリの中で見分けが付くことだけです。
const databaseFileName = "groobb.sqlite"

// schemaSnapshot holds an empty database file that has every migration applied,
// built once per test binary and copied for each test that asks for a database.
//
// [Ja] schemaSnapshot は、マイグレーションをすべて適用した空のデータベースファイルを
// 保持します。テストバイナリごとに 1 度だけ構築し、データベースを要求するテストごとに
// 複製します。
var (
	schemaSnapshot     []byte
	schemaSnapshotErr  error
	schemaSnapshotOnce sync.Once
)

// SetupDB gives the test a migrated SQLite database of its own and returns its
// connection pools, closing them when the test finishes.
//
// Each test gets a separate database file instead of sharing one and rolling
// back a transaction per test, because SQLite allows a single writer at a time
// for the whole file: parallel tests each holding a write transaction would
// serialize on that one writer. The database is a file rather than an in-memory
// one so that tests exercise the WAL mode and lock contention that production
// runs on.
//
// [Ja] SetupDB はテストに専用のマイグレーション済み SQLite データベースを与え、その
// 接続プールを返します。プールはテストの終了時にクローズします。
//
// 1 つのデータベースを共有してテストごとにトランザクションをロールバックするのではなく、
// テストごとにファイルを分けるのは、SQLite がファイル全体で同時に 1 つのライターしか
// 許さないためです。並行するテストがそれぞれ書き込みトランザクションを保持すると、その
// 1 つのライター上で直列化されてしまいます。インメモリではなくファイルにするのは、本番が
// 動作するのと同じ WAL モードとロック競合をテストでも通るようにするためです。
func SetupDB(t *testing.T) *database.DB {
	t.Helper()

	path := SetupDBPath(t)

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to open the test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close the test database: %v", err)
		}
	})

	return db
}

// SetupDBPath gives the test a migrated SQLite database file of its own and
// returns its path, for a caller that opens the file itself rather than taking
// the pools SetupDB returns (the worker client, which owns its own connection).
//
// [Ja] SetupDBPath はテストに専用のマイグレーション済み SQLite データベースファイルを
// 与え、そのパスを返します。SetupDB が返すプールを受け取るのではなく、ファイルを自分で
// 開く呼び出し元 (自身の接続を所有するワーカークライアント) のためのものです。
func SetupDBPath(t *testing.T) string {
	t.Helper()

	snapshot, err := prepareSchemaSnapshot()
	if err != nil {
		t.Fatalf("failed to prepare the test database schema: %v", err)
	}

	// t.TempDir gives each test a directory that is removed when it ends, which
	// takes the database file with it. Its removal is registered before any
	// cleanup the caller adds, so it runs after the caller closes what it opened.
	//
	// [Ja] t.TempDir はテストごとに、そのテストの終了時に削除されるディレクトリを
	// 与える。データベースファイルもそれと共に消える。ディレクトリの削除は呼び出し元が
	// 登録する cleanup より先に登録されるため、実行は呼び出し元が開いたものを閉じた後になる。
	path := filepath.Join(t.TempDir(), databaseFileName)
	if err := os.WriteFile(path, snapshot, 0o600); err != nil {
		t.Fatalf("failed to write the test database file: %v", err)
	}

	return path
}

// prepareSchemaSnapshot returns the snapshot every test database is copied
// from, building it on the first call.
//
// The first call also lowers the bcrypt cost, which is a package-level variable
// that parallel tests would otherwise race to write. Both belong to the same
// one-time setup: a test that calls SetupDB is a test that runs against the
// database, and hashing a password at the production cost dominates such a
// test's runtime.
//
// [Ja] prepareSchemaSnapshot は、すべてのテスト用データベースの複製元となる
// スナップショットを返します。最初の呼び出しで構築します。
//
// 最初の呼び出しでは bcrypt のコストも下げます。これはパッケージレベルの変数であり、
// そうしなければ並行するテストが競合して書き込むことになります。どちらも同じ 1 度きりの
// セットアップに属します。SetupDB を呼ぶテストはデータベースを相手にするテストであり、
// 本番のコストでパスワードをハッシュ化するとそのテストの実行時間の大半を占めるためです。
func prepareSchemaSnapshot() ([]byte, error) {
	schemaSnapshotOnce.Do(func() {
		lowerBcryptCost()

		schemaSnapshot, schemaSnapshotErr = buildSchemaSnapshot()
	})

	return schemaSnapshot, schemaSnapshotErr
}

// buildSchemaSnapshot migrates a throwaway database and returns the resulting
// file.
//
// Copying this snapshot is how each test gets its schema, rather than migrating
// its database directly: writing the file costs about a fifth of what applying
// the migrations does, and the gap widens as migrations accumulate, since goose
// reads and applies all of them every time.
//
// The snapshot is kept in memory rather than as a file on disk because such a
// file would have to outlive every test that copies it, and a test binary has
// no point at which it can remove it afterwards.
//
// [Ja] buildSchemaSnapshot は使い捨てのデータベースをマイグレートし、その結果の
// ファイルを返します。
//
// 各テストがスキーマを得る手段を、データベースを直接マイグレートすることではなく、この
// スナップショットの複製にしているのは、ファイルの書き出しにかかるコストが
// マイグレーションの適用の約 5 分の 1 であり、しかも goose が毎回すべてのマイグレーションを
// 読んで適用する以上、その差はマイグレーションが増えるほど開いていくためです。
//
// スナップショットをディスク上のファイルではなくメモリに持つのは、ファイルにすると
// それを複製するすべてのテストより長く生存させる必要があり、テストバイナリには後から
// それを削除できる地点が無いためです。
func buildSchemaSnapshot() ([]byte, error) {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "groobb-schema-snapshot-")
	if err != nil {
		return nil, fmt.Errorf("failed to create the snapshot directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, databaseFileName)

	db, err := database.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open the snapshot database: %w", err)
	}

	if err := database.Migrate(ctx, db.Writer); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to migrate the snapshot database: %w", err)
	}

	// Close before reading the file: in WAL mode the migrations live in the
	// write-ahead log, and SQLite folds them into the database file when the
	// last connection closes. Reading it earlier would yield a file whose copy
	// is an empty database.
	//
	// [Ja] ファイルを読む前にクローズする。WAL モードではマイグレーションは
	// write-ahead log 上にあり、SQLite が最後のコネクションのクローズでそれを
	// データベースファイルへ畳み込むため。それより早く読むと、複製しても空の
	// データベースにしかならないファイルが得られる。
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("failed to close the snapshot database: %w", err)
	}

	// The path is the temporary directory this function just created joined with
	// a constant file name, so it never comes from outside, and gosec G304 (a
	// file read from a variable path) is a false positive here.
	//
	// [Ja] path は本関数が今作った一時ディレクトリと定数のファイル名を結合したもので、
	// 外部から渡されることはないため、gosec G304 (変数のパスからのファイル読み込みの
	// 指摘) はここでは false positive である。
	//nolint:gosec // G304
	snapshot, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read the snapshot database: %w", err)
	}

	return snapshot, nil
}
