package testutil_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/testutil"
)

// insertUserは書き込み用プールからユーザーを挿入します。エラー時はテストを
// 失敗させます。
func insertUser(t *testing.T, db *database.DB, atname string) {
	t.Helper()

	_, err := db.Writer.ExecContext(
		context.Background(),
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		atname+"@example.com", atname, "ja", "Asia/Tokyo",
	)
	if err != nil {
		t.Fatalf("ユーザーの挿入に失敗: %v", err)
	}
}

// countUsersはデータベースが持つユーザーの件数を、読み取り用プールから読んで
// 返します。
func countUsers(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("ユーザー数の取得に失敗: %v", err)
	}

	return count
}

// TestSetupDB_ReturnsAMigratedDatabaseは、返されるデータベースがスキーマを
// 適用済みで、書き込みも読み取りもできることを検証します。これによりテストは自分で
// マイグレートすることなくデータベースを使えます。
func TestSetupDB_ReturnsAMigratedDatabase(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	insertUser(t, db, "user")

	if got := countUsers(t, db); got != 1 {
		t.Errorf("count(*) FROM users = %d、期待値 = 1", got)
	}
}

// TestSetupDB_GivesEachTestItsOwnDatabaseは、同じテストから得た2つの
// データベースが互いに独立していることを検証します。これがあるからこそ、並行して
// 走るテストがそれぞれテーブルの中身に対する期待値を書けます。
func TestSetupDB_GivesEachTestItsOwnDatabase(t *testing.T) {
	t.Parallel()

	first := testutil.SetupDB(t)
	second := testutil.SetupDB(t)

	insertUser(t, first, "user")

	if got := countUsers(t, second); got != 0 {
		t.Errorf("2つ目のデータベースのcount(*) FROM users = %d、期待値 = 0", got)
	}
}

// TestSetupDB_RejectsWritesThroughTheReadPoolは、テスト用データベースの
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
		t.Error("読み取りプール経由の書き込みが成功した (エラーを期待)")
	}
}

// TestSetupDB_ClosesTheDatabaseWhenTheTestEndsは、プールを要求したテストが
// 終わった時点でプールがクローズされることを検証します。これによりパッケージ内の
// テストが、開いたままのコネクションや一時ファイルを溜め込むことはありません。
func TestSetupDB_ClosesTheDatabaseWhenTheTestEnds(t *testing.T) {
	t.Parallel()

	var db *database.DB
	t.Run("内側のテスト", func(t *testing.T) {
		db = testutil.SetupDB(t)
	})

	// database/sqlはクローズ済みのプールが返すエラーを公開していないため、
	// pingが失敗することだけを検証する。
	if err := db.Writer.PingContext(context.Background()); err == nil {
		t.Error("テスト終了後の書き込みプールへのpingが成功した (エラーを期待)")
	}
	if err := db.Reader.PingContext(context.Background()); err == nil {
		t.Error("テスト終了後の読み取りプールへのpingが成功した (エラーを期待)")
	}
}

// TestSetupDB_LowersTheBcryptCostは、テスト用データベースを要求すると
// ハッシュ化のコストも下がることを検証します。そうでなければ、パスワードを保存する
// テストは実行時間の大半をハッシュ化に費やすことになります。
func TestSetupDB_LowersTheBcryptCost(t *testing.T) {
	t.Parallel()

	testutil.SetupDB(t)

	if auth.BcryptCost != auth.TestBcryptCost {
		t.Errorf("auth.BcryptCost = %d、期待値 = %d", auth.BcryptCost, auth.TestBcryptCost)
	}
}
