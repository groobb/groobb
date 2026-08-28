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

// latestMigratedTableNames are the tables the most recent migration creates,
// for the content of the community. The rollback test reads this as what the
// last migration owns, so adding a migration after this one means moving its
// tables here and leaving these ones in earlierMigratedTableNames.
//
// [Ja] latestMigratedTableNames は、最新のマイグレーションがコミュニティの中身のために
// 作るテーブルです。ロールバックのテストはこれを「最後のマイグレーションが所有するもの」
// として読むため、このあとにマイグレーションを追加するときは、そのテーブルをここへ移し、
// これらを earlierMigratedTableNames へ残します。
var latestMigratedTableNames = []string{
	"boards",
	"categories",
	"post_references",
	"posts",
	"threads",
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

var earlierMigratedTableNames = []string{
	"communities",
	"email_confirmations",
	"password_reset_tokens",
	"roles",
	"user_passwords",
	"user_roles",
	"user_sessions",
	"user_two_factor_auths",
	"users",
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

	for _, table := range slices.Concat(earlierMigratedTableNames, latestMigratedTableNames) {
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
// the most recent migration created, and only that: the migrations applied
// before it are left in place.
//
// [Ja] TestRollback_RevertsTheLastMigration は、ロールバックが最新のマイグレーションの
// 作ったものを取り消すこと、そしてそれだけを取り消すこと (それより前に適用された
// マイグレーションはそのまま残ること) を検証します。
func TestRollback_RevertsTheLastMigration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	if err := database.Rollback(ctx, db.Writer); err != nil {
		t.Fatalf("failed to roll back the migration: %v", err)
	}

	for _, table := range latestMigratedTableNames {
		var name string
		err := db.Reader.QueryRowContext(
			ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name)
		if err == nil {
			t.Errorf("table %q should be gone after rolling back, but it is still there", table)
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("checking table %q after rolling back returned an unexpected error: %v", table, err)
		}
	}

	for _, table := range slices.Concat(earlierMigratedTableNames, riverMigratedTableNames) {
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
