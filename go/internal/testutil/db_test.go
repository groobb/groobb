package testutil_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/testutil"
)

// insertUser inserts a user through the write pool, failing the test on error.
//
// [Ja] insertUser は書き込み用プールからユーザーを挿入します。エラー時はテストを
// 失敗させます。
func insertUser(t *testing.T, db *database.DB, atname string) {
	t.Helper()

	_, err := db.Writer.ExecContext(
		context.Background(),
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		atname+"@example.com", atname, "ja", "Asia/Tokyo",
	)
	if err != nil {
		t.Fatalf("failed to insert a user: %v", err)
	}
}

// countUsers returns how many users the database holds, read through the read
// pool.
//
// [Ja] countUsers はデータベースが持つユーザーの件数を、読み取り用プールから読んで
// 返します。
func countUsers(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("failed to count the users: %v", err)
	}

	return count
}

// TestSetupDB_ReturnsAMigratedDatabase verifies that the returned database has
// the schema applied and is writable and readable, so a test can use it without
// migrating anything itself.
//
// [Ja] TestSetupDB_ReturnsAMigratedDatabase は、返されるデータベースがスキーマを
// 適用済みで、書き込みも読み取りもできることを検証します。これによりテストは自分で
// マイグレートすることなくデータベースを使えます。
func TestSetupDB_ReturnsAMigratedDatabase(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	insertUser(t, db, "user")

	if got := countUsers(t, db); got != 1 {
		t.Errorf("count(*) FROM users = %d, want 1", got)
	}
}

// TestSetupDB_GivesEachTestItsOwnDatabase verifies that two databases obtained
// from the same test are independent, which is what lets tests share table
// contents' expectations while running in parallel.
//
// [Ja] TestSetupDB_GivesEachTestItsOwnDatabase は、同じテストから得た 2 つの
// データベースが互いに独立していることを検証します。これがあるからこそ、並行して
// 走るテストがそれぞれテーブルの中身に対する期待値を書けます。
func TestSetupDB_GivesEachTestItsOwnDatabase(t *testing.T) {
	t.Parallel()

	first := testutil.SetupDB(t)
	second := testutil.SetupDB(t)

	insertUser(t, first, "user")

	if got := countUsers(t, second); got != 0 {
		t.Errorf("count(*) FROM users on the second database = %d, want 0", got)
	}
}

// TestSetupDB_RejectsWritesThroughTheReadPool verifies that the read pool of a
// test database keeps the read-only guard the application's pools have, so a
// misrouted write fails in tests the same way it would in production.
//
// [Ja] TestSetupDB_RejectsWritesThroughTheReadPool は、テスト用データベースの
// 読み取り用プールが、アプリケーションのプールと同じ読み取り専用の防御を保っている
// ことを検証します。これにより、誤って振り分けられた書き込みは本番と同じように
// テストでも失敗します。
func TestSetupDB_RejectsWritesThroughTheReadPool(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	_, err := db.Reader.ExecContext(
		context.Background(),
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	)
	if err == nil {
		t.Error("writing through the read pool succeeded, want an error")
	}
}

// TestSetupDB_ClosesTheDatabaseWhenTheTestEnds verifies that the pools are
// closed once the test that asked for them finishes, so a package's tests do
// not accumulate open connections and temporary files.
//
// [Ja] TestSetupDB_ClosesTheDatabaseWhenTheTestEnds は、プールを要求したテストが
// 終わった時点でプールがクローズされることを検証します。これによりパッケージ内の
// テストが、開いたままのコネクションや一時ファイルを溜め込むことはありません。
func TestSetupDB_ClosesTheDatabaseWhenTheTestEnds(t *testing.T) {
	t.Parallel()

	var db *database.DB
	t.Run("inner", func(t *testing.T) {
		db = testutil.SetupDB(t)
	})

	// database/sql does not export the error a closed pool returns, so the
	// assertion is only that pinging fails.
	//
	// [Ja] database/sql はクローズ済みのプールが返すエラーを公開していないため、
	// ping が失敗することだけを検証する。
	if err := db.Writer.PingContext(context.Background()); err == nil {
		t.Error("pinging the write pool after the test ended succeeded, want an error")
	}
	if err := db.Reader.PingContext(context.Background()); err == nil {
		t.Error("pinging the read pool after the test ended succeeded, want an error")
	}
}

// TestSetupDB_LowersTheBcryptCost verifies that asking for a test database also
// lowers the hashing cost, since a test that stores a password would otherwise
// spend most of its runtime hashing it.
//
// [Ja] TestSetupDB_LowersTheBcryptCost は、テスト用データベースを要求すると
// ハッシュ化のコストも下がることを検証します。そうでなければ、パスワードを保存する
// テストは実行時間の大半をハッシュ化に費やすことになります。
func TestSetupDB_LowersTheBcryptCost(t *testing.T) {
	t.Parallel()

	testutil.SetupDB(t)

	if auth.BcryptCost != auth.TestBcryptCost {
		t.Errorf("auth.BcryptCost = %d, want %d", auth.BcryptCost, auth.TestBcryptCost)
	}
}
