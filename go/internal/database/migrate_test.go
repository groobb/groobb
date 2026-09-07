package database_test

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
)

// migratedTableNames are the application's own tables. The most recent
// migration creates none of them: it replaces an index on posts, so what
// rolling it back undoes is checked through the index names below rather than
// through a table disappearing. A later migration that creates tables gives them
// a list of their own, which the rollback test reads as what the last migration
// owns, leaving these here.
//
// [Ja] migratedTableNames はアプリケーション自身のテーブルです。最新のマイグレーションは
// このどれも作りません。posts の索引を差し替えるものであるため、それをロールバックすると
// 何が戻るのかは、テーブルが消えることではなく後述の索引名で確かめます。この後にテーブルを
// 作るマイグレーションを足すときは、そのテーブルに専用の一覧を与えます。ロールバックの
// テストはそれを「最後のマイグレーションが所有するもの」として読み、これらはここに残ります。
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

// latestMigratedIndexName is the index the most recent migration creates and
// replacedIndexName the one it drops in the same breath, which together are the
// whole of what that migration owns.
//
// [Ja] latestMigratedIndexName は最新のマイグレーションが作る索引、replacedIndexName は
// それが同時に落とす索引で、この 2 つがそのマイグレーションの所有するもののすべてです。
const (
	latestMigratedIndexName = "index_posts_on_user_id_and_created_at_and_id"
	replacedIndexName       = "index_posts_on_user_id"
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

	for _, table := range migratedTableNames {
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

	if !hasIndex(t, db, latestMigratedIndexName) {
		t.Errorf("index %q is missing after migrating", latestMigratedIndexName)
	}
	if hasIndex(t, db, replacedIndexName) {
		t.Errorf("index %q should be gone after migrating, but it is still there", replacedIndexName)
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
// the most recent migration did, and only that: the index it replaced comes
// back, its own goes away, and every table the migrations before it created
// stays where it is.
//
// [Ja] TestRollback_RevertsTheLastMigration は、ロールバックが最新のマイグレーションの
// 行ったことを取り消すこと、そしてそれだけを取り消すことを検証します。差し替えられた索引が
// 戻り、自身が作った索引が消え、それより前のマイグレーションが作ったテーブルはどれもその
// ままです。
func TestRollback_RevertsTheLastMigration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	if err := database.Rollback(ctx, db.Writer); err != nil {
		t.Fatalf("failed to roll back the migration: %v", err)
	}

	if hasIndex(t, db, latestMigratedIndexName) {
		t.Errorf("index %q should be gone after rolling back, but it is still there", latestMigratedIndexName)
	}
	if !hasIndex(t, db, replacedIndexName) {
		t.Errorf("index %q should be back after rolling back, but it is missing", replacedIndexName)
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

	if err := database.Rollback(ctx, db.Writer); err != nil {
		t.Fatalf("failed to roll back the migration: %v", err)
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
	if !hasIndex(t, db, latestMigratedIndexName) {
		t.Errorf("index %q is missing after applying the migration again", latestMigratedIndexName)
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
