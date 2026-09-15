package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// migratedTableNamesは、最新のマイグレーションより前から存在するアプリケーション
// 自身のテーブルです。ロールバックのテストはこれを「最新のマイグレーションを戻しても
// 残るもの」として読みます。そのマイグレーション自身が作るものは
// lastMigratedTableNamesに分けて挙げています。
var migratedTableNames = []string{
	"boards",
	"categories",
	"communities",
	"email_confirmations",
	"password_reset_tokens",
	"post_references",
	"posts",
	"roles",
	"threads",
	"user_passwords",
	"user_roles",
	"user_sessions",
	"user_two_factor_auths",
	"users",
}

// lastMigratedTableNamesとlastMigratedColumnsは、最新のマイグレーションが所有する
// ものです。モデレーションの履歴を保持するテーブルと、既にあったテーブルへ足す4つの
// 状態の列です。ロールバックのテストはこの両方を「1つ戻すと無くなるもの」として読み、
// スキーマのテストはmigratedTableNamesと併せて「マイグレート済みのデータベースが持つ
// もの」として読みます。
var (
	lastMigratedTableNames = []string{"moderation_logs"}

	lastMigratedColumns = []struct {
		table  string
		column string
	}{
		{table: "threads", column: "locked_at"},
		{table: "threads", column: "unpublished_at"},
		{table: "posts", column: "unpublished_at"},
		{table: "users", column: "suspended_at"},
	}
)

// riverMigratedTableNamesは、バックグラウンドジョブキューRiverのために
// マイグレートされるテーブルです。中身を所有するのがGroobbではなくRiverであるため、
// アプリケーション自身のテーブルとは分けています。
var riverMigratedTableNames = []string{
	"river_job",
	"river_leader",
	"river_migration",
	"river_notification",
	"river_queue",
}

// postAuthorIndexNameは作者の最新の投稿を読むための索引、
// replacedPostAuthorIndexNameはそれが取って代わった索引で、この2つが最後から1つ前の
// マイグレーションの所有するもののすべてです。
const (
	postAuthorIndexName         = "index_posts_on_user_id_and_created_at_and_id"
	replacedPostAuthorIndexName = "index_posts_on_user_id"
)

// hasIndexは、スキーマが現在その名前の索引を持っているかを返します。
func hasIndex(t *testing.T, db *database.DB, name string) bool {
	t.Helper()

	var found string
	err := db.Reader.QueryRowContext(
		context.Background(),
		"SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?",
		name,
	).Scan(&found)
	if err == nil {
		return true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("索引 %q の確認に失敗: %v", name, err)
	}

	return false
}

// hasColumnは、そのテーブルが現在その名前の列を持っているかを返します。テーブルの
// 宣言文ではなくpragma_table_infoを読むのは、別の列の名前の中や既定値の式の中に現れた
// 名前を、その列そのものと取り違えないためです。
func hasColumn(t *testing.T, db *database.DB, table, column string) bool {
	t.Helper()

	var found string
	err := db.Reader.QueryRowContext(
		context.Background(),
		"SELECT name FROM pragma_table_info(?) WHERE name = ?",
		table, column,
	).Scan(&found)
	if err == nil {
		return true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("列 %s.%s の確認に失敗: %v", table, column, err)
	}

	return false
}

// migratedTestDBは使い捨てのデータベースを開き、最新のマイグレーションまで
// 適用した状態にします。
func migratedTestDB(t *testing.T) *database.DB {
	t.Helper()

	db, _ := openTestDB(t)

	if err := database.Migrate(context.Background(), db.Writer); err != nil {
		t.Fatalf("データベースのマイグレーションに失敗: %v", err)
	}

	return db
}

// TestMigrate_CreatesTheSchemaは、空のデータベースをマイグレートすると
// アプリケーションが期待するテーブルが作られることを検証します。
func TestMigrate_CreatesTheSchema(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	for _, table := range slices.Concat(migratedTableNames, lastMigratedTableNames) {
		var name string
		err := db.Reader.QueryRowContext(
			context.Background(),
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("マイグレーション後にテーブル %q が無い: %v", table, err)
		}
	}
}

// TestMigrate_AddsTheModerationColumnsは、マイグレート済みのデータベースが、管理者の
// 判断を保持する列を持つことを検証します。リポジトリ経由だけでなくここで確かめるのは、
// 既に行を持つテーブルへの列の追加が、データの入ったデータベースで失敗しうる部分である
// ためです。
func TestMigrate_AddsTheModerationColumns(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	for _, c := range lastMigratedColumns {
		if !hasColumn(t, db, c.table, c.column) {
			t.Errorf("マイグレーション後に列 %s.%s が無い", c.table, c.column)
		}
	}
}

// TestMigrate_ReplacesTheIndexOnThePostAuthorは、マイグレート済みのデータベースが、
// 作者の最新の投稿を読むための索引を持ち、それが差し替えた索引はもう持たないことを検証
// します。これにより先頭のカラムが二重に索引付けされることはなく、書き込まれる投稿が
// 支払うのは2つではなく1つの索引の更新になります。
func TestMigrate_ReplacesTheIndexOnThePostAuthor(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	if !hasIndex(t, db, postAuthorIndexName) {
		t.Errorf("マイグレーション後に索引 %q が無い", postAuthorIndexName)
	}
	if hasIndex(t, db, replacedPostAuthorIndexName) {
		t.Errorf("マイグレーション後に索引 %q は無いはずだが、まだある", replacedPostAuthorIndexName)
	}
}

// TestMigrate_InsertsTheAdminRoleは、マイグレート済みのデータベースが、他のすべてを
// 含意するスコープを持つ組み込みのadminロールを既に保持していることを検証します。これに
// より、誰もサインインしていないインスタンスにも最初の管理者を立てられます。
func TestMigrate_InsertsTheAdminRole(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	var scopesJSON string
	if err := db.Reader.QueryRowContext(
		context.Background(),
		"SELECT CAST(scopes AS TEXT) FROM roles WHERE name = ?",
		string(model.RoleNameAdmin),
	).Scan(&scopesJSON); err != nil {
		t.Fatalf("マイグレーション後に %q ロールが無い: %v", model.RoleNameAdmin, err)
	}

	var scopes []model.Scope
	if err := json.Unmarshal([]byte(scopesJSON), &scopes); err != nil {
		t.Fatalf("%q ロールのscopesがJSON配列ではない: %v", model.RoleNameAdmin, err)
	}

	want := []model.Scope{model.ScopeCommunityAdmin}
	if !slices.Equal(scopes, want) {
		t.Errorf("%q ロールのscopes = %v、期待値 = %v", model.RoleNameAdmin, scopes, want)
	}
}

// TestMigrate_IsIdempotentは、最新の状態のデータベースをマイグレートしても
// 何も起きないことを検証します。これにより呼び出し側は現在のバージョンを先に確認せず、
// 安全にマイグレーションを適用できます。
func TestMigrate_IsIdempotent(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	if err := database.Migrate(context.Background(), db.Writer); err != nil {
		t.Fatalf("最新の状態のデータベースのマイグレーションがエラーを返した: %v", err)
	}
}

// TestMigrate_TimestampDefaultsAreFixedWidthは、データベースが書き込む既定値が、
// アプリケーションが自身の時刻を束縛するのと同じ桁数固定のISO8601書式であることを
// 検証します。SQLiteは時刻をテキストとして比較するため、桁数や区切りの異なる既定値は
// 失敗を起こさないまま行の順序を狂わせます。
func TestMigrate_TimestampDefaultsAreFixedWidth(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	); err != nil {
		t.Fatalf("ユーザーの挿入に失敗: %v", err)
	}

	// 保存されているテキストがそのまま届くのはCASTがあるためです。宣言型が
	// DATETIMEの列はドライバからtime.Timeとして返り、それをstringに受けると
	// database/sqlがRFC3339Nanoで整形するため、本テストが確かめたい末尾のゼロが
	// 落ちます (".970Z" が ".97Z" として届く)。CASTは式であり宣言型を持たないため、
	// ドライバはSQLiteが保持しているテキストをそのまま渡します。
	var createdAt string
	if err := db.Reader.QueryRowContext(ctx, "SELECT CAST(created_at AS TEXT) FROM users").Scan(&createdAt); err != nil {
		t.Fatalf("タイムスタンプの読み戻しに失敗: %v", err)
	}

	if _, err := time.Parse("2006-01-02T15:04:05.000Z", createdAt); err != nil {
		t.Errorf("created_at = %q が期待する書式ではない: %v", createdAt, err)
	}
}

// TestMigrate_UserUniquenessIgnoresLetterCaseは、users.emailとusers.atnameの
// 照合順序により大文字小文字だけが異なる値が衝突することを検証します。これにより1つの
// アドレスや1つのハンドルが2つのアカウントになることはありません。
func TestMigrate_UserUniquenessIgnoresLetterCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		email  string
		atname string
	}{
		{name: "email", email: "USER@example.com", atname: "other"},
		{name: "atname", email: "other@example.com", atname: "USER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := migratedTestDB(t)

			if _, err := db.Writer.ExecContext(
				ctx,
				"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
				"user@example.com", "user", "ja", "Asia/Tokyo",
			); err != nil {
				t.Fatalf("ユーザーの挿入に失敗: %v", err)
			}

			if _, err := db.Writer.ExecContext(
				ctx,
				"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
				tt.email, tt.atname, "ja", "Asia/Tokyo",
			); err == nil {
				t.Errorf("大文字小文字だけが異なる同じ %s の挿入は失敗するはずだが、成功した", tt.name)
			}
		})
	}
}

// TestRollback_RevertsTheLastMigrationは、ロールバックが最新のマイグレーションの
// 行ったことを取り消すこと、そしてそれだけを取り消すことを検証します。モデレーションの
// 履歴と4つの状態の列が消え、その1つ前のマイグレーションが挿入した組み込みのadminロール、
// さらに1つ前が残した索引、そしてそれらより前のマイグレーションが作ったテーブルはどれも
// そのままです。
func TestRollback_RevertsTheLastMigration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	if err := database.Rollback(ctx, db.Writer); err != nil {
		t.Fatalf("マイグレーションのロールバックに失敗: %v", err)
	}

	for _, table := range lastMigratedTableNames {
		var name string
		err := db.Reader.QueryRowContext(
			ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("最新のマイグレーションのロールバック後にテーブル %q は無いはずだが、まだある", table)
		}
	}
	for _, c := range lastMigratedColumns {
		if hasColumn(t, db, c.table, c.column) {
			t.Errorf("最新のマイグレーションのロールバック後に列 %s.%s は無いはずだが、まだある", c.table, c.column)
		}
	}

	if count := countRows(t, db, "SELECT COUNT(*) FROM roles WHERE name = ?", string(model.RoleNameAdmin)); count != 1 {
		t.Errorf("ロールバック後に残る %q ロールの行数 = %d、期待値 = 1", model.RoleNameAdmin, count)
	}
	if !hasIndex(t, db, postAuthorIndexName) {
		t.Errorf("最新のマイグレーションのロールバック後も索引 %q は残るはずだが、無い", postAuthorIndexName)
	}

	for _, table := range slices.Concat(migratedTableNames, riverMigratedTableNames) {
		var name string
		if err := db.Reader.QueryRowContext(
			ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name); err != nil {
			t.Errorf("最新のマイグレーションのロールバック後もテーブル %q は残るはずだが、確認に失敗: %v", table, err)
		}
	}
}

// TestMigrate_KeepsThePostsAcrossTheIndexReplacementは、投稿を持つデータベースが
// 索引のマイグレーションを両方向に越えても、行がそのままであることを検証します。索引の
// 差し替えは行を書き換えず、前のバージョンへ戻してからまた進めることになったインスタンスも、
// その途中で書かれたものを失いません。
func TestMigrate_KeepsThePostsAcrossTheIndexReplacement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	// どこまで戻すかは、マイグレーションの本数ではなく索引で決めます。索引の
	// マイグレーションはもう最後のものではなく、この後に足されるマイグレーションの数だけ
	// 奥へ下がっていくためです。本数で書けば、本テストは目的の状態に届かないまま通り
	// 続けます。索引のマイグレーションを越えていないデータベースも、投稿は同じように
	// 保つためです。
	for hasIndex(t, db, postAuthorIndexName) {
		if err := database.Rollback(ctx, db.Writer); err != nil {
			t.Fatalf("マイグレーションのロールバックに失敗: %v", err)
		}
	}

	// 前の索引が戻っている間に書かれた投稿は、インスタンスがそこから先へ
	// マイグレートする状態そのものである。
	rolledBackPostID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, "索引を戻している間に書いた投稿",
	)

	if err := database.Migrate(ctx, db.Writer); err != nil {
		t.Fatalf("マイグレーションの再適用に失敗: %v", err)
	}

	for _, postID := range []int64{ids.postID, rolledBackPostID} {
		if count := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ?", postID); count != 1 {
			t.Errorf("マイグレーションを往復した後に残る投稿 %d の行数 = %d、期待値 = 1", postID, count)
		}
	}
	if !hasIndex(t, db, postAuthorIndexName) {
		t.Errorf("マイグレーションの再適用後に索引 %q が無い", postAuthorIndexName)
	}
}

// TestMigrate_KeepsTheContentAcrossTheModerationColumnsは、コミュニティの書き込みを
// 持つデータベースがモデレーションのマイグレーションを両方向に越えても、行がそのままで
// あることを検証します。
//
// ここがこのマイグレーションで何かを失いうる向きです。SQLiteは既に行を持つテーブルに対して
// 列を足し、また落とします。前のバージョンへ戻してからまた進めることになったインスタンスは、
// スレッドと投稿がそのまま残っていなければなりません。列が持っていたものが戻ることは期待
// しません (ロールバックは列ごと取り除くため、そこに記録された判断は失われます)。戻るのは、
// それらが向けられていた書き込みのほうです。
func TestMigrate_KeepsTheContentAcrossTheModerationColumns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if err := database.Rollback(ctx, db.Writer); err != nil {
		t.Fatalf("マイグレーションのロールバックに失敗: %v", err)
	}

	// 列が無い間に書かれた投稿は、インスタンスがそこから先へマイグレートする状態
	// そのものである。
	rolledBackPostID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, "列を戻している間に書いた投稿",
	)

	if err := database.Migrate(ctx, db.Writer); err != nil {
		t.Fatalf("マイグレーションの再適用に失敗: %v", err)
	}

	if count := countRows(t, db, "SELECT COUNT(*) FROM threads WHERE id = ?", ids.threadID); count != 1 {
		t.Errorf("マイグレーションを往復した後に残るスレッドの行数 = %d、期待値 = 1", count)
	}
	for _, postID := range []int64{ids.postID, rolledBackPostID} {
		if count := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ?", postID); count != 1 {
			t.Errorf("マイグレーションを往復した後に残る投稿 %d の行数 = %d、期待値 = 1", postID, count)
		}
	}
	for _, c := range lastMigratedColumns {
		if !hasColumn(t, db, c.table, c.column) {
			t.Errorf("マイグレーションの再適用後に列 %s.%s が無い", c.table, c.column)
		}
	}
}

// TestRollback_WithoutAppliedMigrationsは、適用済みのものが無いデータベースを
// ロールバックしたときに、黙って成功せずエラーを返すことを検証します。
func TestRollback_WithoutAppliedMigrations(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if err := database.Rollback(context.Background(), db.Writer); err == nil {
		t.Error("適用済みのものが無い状態でのロールバックは失敗するはずだが、成功した")
	}
}
