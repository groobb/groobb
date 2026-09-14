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

// migratedTableNames are the application's own tables that predate the most
// recent migration, so the rollback test reads them as what has to survive
// rolling it back. What that migration itself creates is listed separately in
// lastMigratedTableNames.
//
// [Ja] migratedTableNamesは、最新のマイグレーションより前から存在するアプリケーション
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

// lastMigratedTableNames and lastMigratedColumns are what the most recent
// migration owns: the table holding the history of moderation, and the four
// state columns it adds to tables that already existed. The rollback test reads
// both as what going back one step takes away, and the schema test reads them
// alongside migratedTableNames as what a migrated database carries.
//
// [Ja] lastMigratedTableNamesとlastMigratedColumnsは、最新のマイグレーションが所有する
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

// riverMigratedTableNames are the tables migrated for River (the background job
// queue). They sit apart from the application's own tables because River, not
// Groobb, owns what they hold.
//
// [Ja] riverMigratedTableNames は、バックグラウンドジョブキュー River のために
// マイグレートされるテーブルです。中身を所有するのが Groobb ではなく River であるため、
// アプリケーション自身のテーブルとは分けています。
var riverMigratedTableNames = []string{
	"river_job",
	"river_leader",
	"river_migration",
	"river_notification",
	"river_queue",
}

// postAuthorIndexName is the index an author's latest post is read through and
// replacedPostAuthorIndexName the one it took the place of, which together are
// the whole of what the migration before the last one owns.
//
// [Ja] postAuthorIndexName は作者の最新の投稿を読むための索引、
// replacedPostAuthorIndexName はそれが取って代わった索引で、この 2 つが最後から 1 つ前の
// マイグレーションの所有するもののすべてです。
const (
	postAuthorIndexName         = "index_posts_on_user_id_and_created_at_and_id"
	replacedPostAuthorIndexName = "index_posts_on_user_id"
)

// hasIndex reports whether the schema currently carries the named index.
//
// [Ja] hasIndex は、スキーマが現在その名前の索引を持っているかを返します。
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
		t.Fatalf("checking index %q failed: %v", name, err)
	}

	return false
}

// hasColumn reports whether the named table currently carries the named column.
// It reads pragma_table_info rather than the table's declaration text, so a
// column name appearing inside another column's name or inside a default
// expression is not mistaken for the column itself.
//
// [Ja] hasColumnは、そのテーブルが現在その名前の列を持っているかを返します。テーブルの
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
		t.Fatalf("checking column %s.%s failed: %v", table, column, err)
	}

	return false
}

// migratedTestDB opens a throwaway database and brings it up to the latest
// migration.
//
// [Ja] migratedTestDB は使い捨てのデータベースを開き、最新のマイグレーションまで
// 適用した状態にします。
func migratedTestDB(t *testing.T) *database.DB {
	t.Helper()

	db, _ := openTestDB(t)

	if err := database.Migrate(context.Background(), db.Writer); err != nil {
		t.Fatalf("failed to migrate the database: %v", err)
	}

	return db
}

// TestMigrate_CreatesTheSchema verifies that migrating an empty database
// creates the tables the application expects.
//
// [Ja] TestMigrate_CreatesTheSchema は、空のデータベースをマイグレートすると
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
			t.Errorf("table %q is missing after migrating: %v", table, err)
		}
	}
}

// TestMigrate_AddsTheModerationColumns verifies that a migrated database carries
// the columns an administrator's decisions are held in. They are checked here
// rather than only through the repositories, because a column added to a table
// that already holds rows is the part of the migration that can fail on a
// database with data in it.
//
// [Ja] TestMigrate_AddsTheModerationColumnsは、マイグレート済みのデータベースが、管理者の
// 判断を保持する列を持つことを検証します。リポジトリ経由だけでなくここで確かめるのは、
// 既に行を持つテーブルへの列の追加が、データの入ったデータベースで失敗しうる部分である
// ためです。
func TestMigrate_AddsTheModerationColumns(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	for _, c := range lastMigratedColumns {
		if !hasColumn(t, db, c.table, c.column) {
			t.Errorf("column %s.%s is missing after migrating", c.table, c.column)
		}
	}
}

// TestMigrate_ReplacesTheIndexOnThePostAuthor verifies that a migrated database
// carries the index an author's latest post is read through and no longer the
// one it replaced, so that the leading column is not indexed twice and every
// post written pays for one index rather than two.
//
// [Ja] TestMigrate_ReplacesTheIndexOnThePostAuthor は、マイグレート済みのデータベースが、
// 作者の最新の投稿を読むための索引を持ち、それが差し替えた索引はもう持たないことを検証
// します。これにより先頭のカラムが二重に索引付けされることはなく、書き込まれる投稿が
// 支払うのは 2 つではなく 1 つの索引の更新になります。
func TestMigrate_ReplacesTheIndexOnThePostAuthor(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	if !hasIndex(t, db, postAuthorIndexName) {
		t.Errorf("index %q is missing after migrating", postAuthorIndexName)
	}
	if hasIndex(t, db, replacedPostAuthorIndexName) {
		t.Errorf("index %q should be gone after migrating, but it is still there", replacedPostAuthorIndexName)
	}
}

// TestMigrate_InsertsTheAdminRole verifies that a migrated database already
// holds the built-in admin role with the scope that implies every other, so that
// an instance can be given its first administrator before anyone has signed in.
//
// [Ja] TestMigrate_InsertsTheAdminRole は、マイグレート済みのデータベースが、他のすべてを
// 含意するスコープを持つ組み込みの admin ロールを既に保持していることを検証します。これに
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
		t.Fatalf("the %q role is missing after migrating: %v", model.RoleNameAdmin, err)
	}

	var scopes []model.Scope
	if err := json.Unmarshal([]byte(scopesJSON), &scopes); err != nil {
		t.Fatalf("scopes of the %q role are not a JSON array: %v", model.RoleNameAdmin, err)
	}

	want := []model.Scope{model.ScopeCommunityAdmin}
	if !slices.Equal(scopes, want) {
		t.Errorf("scopes of the %q role = %v, want %v", model.RoleNameAdmin, scopes, want)
	}
}

// TestMigrate_IsIdempotent verifies that migrating an up-to-date database is a
// no-op, so callers can safely apply migrations without first checking the
// current version.
//
// [Ja] TestMigrate_IsIdempotent は、最新の状態のデータベースをマイグレートしても
// 何も起きないことを検証します。これにより呼び出し側は現在のバージョンを先に確認せず、
// 安全にマイグレーションを適用できます。
func TestMigrate_IsIdempotent(t *testing.T) {
	t.Parallel()

	db := migratedTestDB(t)

	if err := database.Migrate(context.Background(), db.Writer); err != nil {
		t.Fatalf("migrating an up-to-date database returned an error: %v", err)
	}
}

// TestMigrate_TimestampDefaultsAreFixedWidth verifies that the default written
// by the database matches the fixed-width ISO8601 format the application binds
// its own timestamps in. SQLite compares timestamps as text, so a default whose
// width or separator differed would order rows wrongly without failing.
//
// [Ja] TestMigrate_TimestampDefaultsAreFixedWidth は、データベースが書き込む既定値が、
// アプリケーションが自身の時刻を束縛するのと同じ桁数固定の ISO8601 書式であることを
// 検証します。SQLite は時刻をテキストとして比較するため、桁数や区切りの異なる既定値は
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
		t.Fatalf("failed to insert a user: %v", err)
	}

	// The cast is what makes the stored text arrive unchanged. A column
	// declared DATETIME comes back from the driver as a time.Time, and
	// database/sql renders that with RFC3339Nano on its way into a string,
	// which drops the trailing zeros this test exists to check (".970Z"
	// arrives as ".97Z"). A cast is an expression and carries no declared
	// type, so the driver hands over the text SQLite holds.
	//
	// [Ja] 保存されているテキストがそのまま届くのは CAST があるためです。宣言型が
	// DATETIME の列はドライバから time.Time として返り、それを string に受けると
	// database/sql が RFC3339Nano で整形するため、本テストが確かめたい末尾のゼロが
	// 落ちます (".970Z" が ".97Z" として届く)。CAST は式であり宣言型を持たないため、
	// ドライバは SQLite が保持しているテキストをそのまま渡します。
	var createdAt string
	if err := db.Reader.QueryRowContext(ctx, "SELECT CAST(created_at AS TEXT) FROM users").Scan(&createdAt); err != nil {
		t.Fatalf("failed to read the timestamp back: %v", err)
	}

	if _, err := time.Parse("2006-01-02T15:04:05.000Z", createdAt); err != nil {
		t.Errorf("created_at = %q, which is not the expected format: %v", createdAt, err)
	}
}

// TestMigrate_UserUniquenessIgnoresLetterCase verifies that the collation on
// users.email and users.atname makes values that differ only in case collide,
// so that one address or one handle cannot become two accounts.
//
// [Ja] TestMigrate_UserUniquenessIgnoresLetterCase は、users.email と users.atname の
// 照合順序により大文字小文字だけが異なる値が衝突することを検証します。これにより 1 つの
// アドレスや 1 つのハンドルが 2 つのアカウントになることはありません。
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
				t.Fatalf("failed to insert a user: %v", err)
			}

			if _, err := db.Writer.ExecContext(
				ctx,
				"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
				tt.email, tt.atname, "ja", "Asia/Tokyo",
			); err == nil {
				t.Errorf("inserting the same %s in a different case should fail, but it succeeded", tt.name)
			}
		})
	}
}

// TestRollback_RevertsTheLastMigration verifies that rolling back undoes what
// the most recent migration did, and only that: the moderation history and the
// four state columns go away, while the built-in admin role the migration
// before it inserted, the index the one before that left in place, and every
// table the migrations before them created stay where they are.
//
// [Ja] TestRollback_RevertsTheLastMigrationは、ロールバックが最新のマイグレーションの
// 行ったことを取り消すこと、そしてそれだけを取り消すことを検証します。モデレーションの
// 履歴と4つの状態の列が消え、その1つ前のマイグレーションが挿入した組み込みのadminロール、
// さらに1つ前が残した索引、そしてそれらより前のマイグレーションが作ったテーブルはどれも
// そのままです。
func TestRollback_RevertsTheLastMigration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	if err := database.Rollback(ctx, db.Writer); err != nil {
		t.Fatalf("failed to roll back the migration: %v", err)
	}

	for _, table := range lastMigratedTableNames {
		var name string
		err := db.Reader.QueryRowContext(
			ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("table %q should be gone after rolling back the last migration, but it is still there", table)
		}
	}
	for _, c := range lastMigratedColumns {
		if hasColumn(t, db, c.table, c.column) {
			t.Errorf("column %s.%s should be gone after rolling back the last migration, but it is still there", c.table, c.column)
		}
	}

	if count := countRows(t, db, "SELECT COUNT(*) FROM roles WHERE name = ?", string(model.RoleNameAdmin)); count != 1 {
		t.Errorf("rows left for the %q role after rolling back = %d, want 1", model.RoleNameAdmin, count)
	}
	if !hasIndex(t, db, postAuthorIndexName) {
		t.Errorf("index %q should survive rolling back the last migration, but it is missing", postAuthorIndexName)
	}

	for _, table := range slices.Concat(migratedTableNames, riverMigratedTableNames) {
		var name string
		if err := db.Reader.QueryRowContext(
			ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name); err != nil {
			t.Errorf("table %q should survive rolling back the last migration, but checking it failed: %v", table, err)
		}
	}
}

// TestMigrate_KeepsThePostsAcrossTheIndexReplacement verifies that a database
// holding posts crosses the index migration in both directions with its rows
// where they were. Replacing an index rewrites no row, and an instance that has
// to step back to the previous version and forward again loses no writing on
// the way.
//
// [Ja] TestMigrate_KeepsThePostsAcrossTheIndexReplacement は、投稿を持つデータベースが
// 索引のマイグレーションを両方向に越えても、行がそのままであることを検証します。索引の
// 差し替えは行を書き換えず、前のバージョンへ戻してからまた進めることになったインスタンスも、
// その途中で書かれたものを失いません。
func TestMigrate_KeepsThePostsAcrossTheIndexReplacement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	// How far back to step is decided by the index rather than by a number of
	// migrations, because the index migration is no longer the last one and each
	// migration added after it moves it one further back. A number would leave
	// this test stepping short of the state it is written for while still
	// passing, since a database that never crossed the index migration keeps its
	// posts just as well.
	//
	// [Ja] どこまで戻すかは、マイグレーションの本数ではなく索引で決めます。索引の
	// マイグレーションはもう最後のものではなく、この後に足されるマイグレーションの数だけ
	// 奥へ下がっていくためです。本数で書けば、本テストは目的の状態に届かないまま通り
	// 続けます。索引のマイグレーションを越えていないデータベースも、投稿は同じように
	// 保つためです。
	for hasIndex(t, db, postAuthorIndexName) {
		if err := database.Rollback(ctx, db.Writer); err != nil {
			t.Fatalf("failed to roll back the migration: %v", err)
		}
	}

	// A post written while the previous index is back in place is the state an
	// instance migrates forward from.
	//
	// [Ja] 前の索引が戻っている間に書かれた投稿は、インスタンスがそこから先へ
	// マイグレートする状態そのものである。
	rolledBackPostID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, "索引を戻している間に書いた投稿",
	)

	if err := database.Migrate(ctx, db.Writer); err != nil {
		t.Fatalf("failed to apply the migration again: %v", err)
	}

	for _, postID := range []int64{ids.postID, rolledBackPostID} {
		if count := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ?", postID); count != 1 {
			t.Errorf("rows left for the post %d after migrating back and forth = %d, want 1", postID, count)
		}
	}
	if !hasIndex(t, db, postAuthorIndexName) {
		t.Errorf("index %q is missing after applying the migration again", postAuthorIndexName)
	}
}

// TestMigrate_KeepsTheContentAcrossTheModerationColumns verifies that a
// database holding a community's writing crosses the moderation migration in
// both directions with its rows where they were.
//
// It is the direction the migration can lose something in: SQLite adds and
// drops a column on a table that already holds rows, and an instance that has
// to step back to the previous version and forward again must find its threads
// and posts still there. What the columns held is not expected back -- rolling
// back removes the column, so the decisions recorded in it are gone -- and the
// writing they were about is.
//
// [Ja] TestMigrate_KeepsTheContentAcrossTheModerationColumnsは、コミュニティの書き込みを
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
		t.Fatalf("failed to roll back the migration: %v", err)
	}

	// A post written while the columns are gone is the state an instance
	// migrates forward from.
	//
	// [Ja] 列が無い間に書かれた投稿は、インスタンスがそこから先へマイグレートする状態
	// そのものである。
	rolledBackPostID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, "列を戻している間に書いた投稿",
	)

	if err := database.Migrate(ctx, db.Writer); err != nil {
		t.Fatalf("failed to apply the migration again: %v", err)
	}

	if count := countRows(t, db, "SELECT COUNT(*) FROM threads WHERE id = ?", ids.threadID); count != 1 {
		t.Errorf("rows left for the thread after migrating back and forth = %d, want 1", count)
	}
	for _, postID := range []int64{ids.postID, rolledBackPostID} {
		if count := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ?", postID); count != 1 {
			t.Errorf("rows left for the post %d after migrating back and forth = %d, want 1", postID, count)
		}
	}
	for _, c := range lastMigratedColumns {
		if !hasColumn(t, db, c.table, c.column) {
			t.Errorf("column %s.%s is missing after applying the migration again", c.table, c.column)
		}
	}
}

// TestRollback_WithoutAppliedMigrations verifies that rolling back a database
// with nothing applied reports an error instead of passing silently.
//
// [Ja] TestRollback_WithoutAppliedMigrations は、適用済みのものが無いデータベースを
// ロールバックしたときに、黙って成功せずエラーを返すことを検証します。
func TestRollback_WithoutAppliedMigrations(t *testing.T) {
	t.Parallel()

	db, _ := openTestDB(t)

	if err := database.Rollback(context.Background(), db.Writer); err == nil {
		t.Error("rolling back with nothing applied should fail, but it succeeded")
	}
}
