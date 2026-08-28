package seed

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestCleanupTablesCoverTheSchema verifies that every table the schema has is
// named by exactly one of the two lists. A table on neither list would survive
// the cleanup without anyone deciding that it should, and leave rows from an
// earlier run behind for the next one.
//
// [Ja] TestCleanupTablesCoverTheSchema は、スキーマの持つすべてのテーブルが 2 つの一覧の
// ちょうど一方に挙げられていることを検証します。どちらの一覧にも無いテーブルは、誰も
// そう決めていないのにクリーンアップを生き延び、前回の実行の行を次回へ残してしまいます。
func TestCleanupTablesCoverTheSchema(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	classified := make(map[string]string, len(cleanupTables)+len(preservedTables))
	for _, group := range []struct {
		name   string
		tables []string
	}{
		{name: "cleanupTables", tables: cleanupTables},
		{name: "preservedTables", tables: preservedTables},
	} {
		for _, table := range group.tables {
			if previous, exists := classified[table]; exists {
				t.Errorf("the table %s is in both %s and %s", table, previous, group.name)

				continue
			}
			classified[table] = group.name
		}
	}

	// The tables SQLite keeps for itself are excluded: they are named
	// sqlite_-something (sqlite_sequence, which the AUTOINCREMENT column of a
	// River table brings along), and they are not the seed's to classify.
	//
	// [Ja] SQLite が自身のために持つテーブルは除外します。これらは sqlite_ で始まる名前を
	// 持ち (River のテーブルの AUTOINCREMENT 列が連れてくる sqlite_sequence など)、シードが
	// 振り分けるものではありません。
	rows, err := db.Reader.QueryContext(
		context.Background(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
	)
	if err != nil {
		t.Fatalf("failed to list the tables: %v", err)
	}
	defer func() { _ = rows.Close() }()

	existing := make(map[string]bool, len(classified))
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("failed to read a table name: %v", err)
		}
		existing[table] = true

		if _, exists := classified[table]; !exists {
			t.Errorf("the table %s is in neither cleanupTables nor preservedTables", table)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to walk the table names: %v", err)
	}

	for table := range classified {
		if !existing[table] {
			t.Errorf("the listed table %s does not exist in the schema", table)
		}
	}
}

// TestCleanup_EmptiesTheTablesItManages verifies that a populated database comes
// out of the cleanup with every managed table empty and the preserved ones
// untouched. The rows are a full chain of the foreign keys involved, because
// what the order of the deletes is there for is to keep a row from being left
// pointing at a row that is gone.
//
// [Ja] TestCleanup_EmptiesTheTablesItManages は、行の入ったデータベースがクリーンアップを
// 経て、管理対象のテーブルはすべて空になり、保護対象のテーブルは手つかずのまま残ることを
// 検証します。投入する行が関係する外部キーを一通りつないだものになっているのは、削除の
// 順序が、消えた行を指したままの行を残さないためにあるからです。
func TestCleanup_EmptiesTheTablesItManages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	populateForCleanup(t, db)

	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin the transaction: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := cleanup(ctx, tx); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit the transaction: %v", err)
	}

	for _, table := range cleanupTables {
		if count := countRows(t, db, table); count != 0 {
			t.Errorf("the table %s holds %d rows after the cleanup, want 0", table, count)
		}
	}

	// roles is the preserved table this test can actually populate, so it is the
	// one that shows the cleanup leaving a preserved table alone. The others hold
	// bookkeeping this test has no way to write.
	//
	// It survives while the user_roles rows pointing at it are emptied, which is
	// what says the cleanup takes the assignments without taking what they were
	// assigning.
	//
	// [Ja] roles は本テストが実際に行を入れられる保護対象テーブルであり、クリーンアップが
	// 保護対象へ手を出さないことを示せるのはこれだけです。他は本テストが書き込む手立てを
	// 持たない管理情報を保持します。
	//
	// roles が残る一方でそれを指す user_roles の行は空になります。これが、クリーンアップが
	// 割り当てを消しても、割り当てていた対象までは消さないことを示します。
	if count := countRows(t, db, "roles"); count != 1 {
		t.Errorf("the roles table holds %d rows after the cleanup, want 1", count)
	}
	if slices.Contains(cleanupTables, "goose_db_version") {
		t.Error("goose_db_version is in cleanupTables; emptying it would make the database look unmigrated")
	}
}

// populateForCleanup writes one row into every table the cleanup manages, tying
// them together through the foreign keys that decide the order of the deletes.
// It also fills the preserved table a test can write to, so that the same run
// shows what the cleanup leaves behind.
//
// [Ja] populateForCleanup は、クリーンアップが管理する各テーブルへ 1 行ずつ書き込み、
// それらを削除の順序を決める外部キーで結び付けます。テストが書き込める保護対象テーブルにも
// 行を入れ、同じ実行でクリーンアップが何を残すのかも示せるようにします。
func populateForCleanup(t *testing.T, db *database.DB) {
	t.Helper()

	ctx := context.Background()

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Groobb"); err != nil {
		t.Fatalf("failed to insert the community: %v", err)
	}

	authorID := testutil.NewUserBuilder(t, db).Build()
	replierID := testutil.NewUserBuilder(t, db).Build()

	testutil.NewUserPasswordBuilder(t, db).WithUserID(authorID).Build()
	testutil.NewUserSessionBuilder(t, db).WithUserID(authorID).Build()
	testutil.NewEmailConfirmationBuilder(t, db).WithUserID(authorID).Build()
	testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(authorID).Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(authorID).Build()

	var roleID int64
	if err := db.Writer.QueryRowContext(
		ctx, "INSERT INTO roles (name) VALUES (?) RETURNING id", "member",
	).Scan(&roleID); err != nil {
		t.Fatalf("failed to insert the role: %v", err)
	}
	if _, err := db.Writer.ExecContext(
		ctx, "INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", int64(authorID), roleID,
	); err != nil {
		t.Fatalf("failed to assign the role: %v", err)
	}

	category, err := repository.NewCategoryRepository(db).Create(ctx, repository.CreateCategoryInput{
		Slug: "announcements", Name: "お知らせ", Position: 1,
	})
	if err != nil {
		t.Fatalf("failed to create the category: %v", err)
	}

	board, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID, Slug: "general", Name: "雑談", Position: 1,
	})
	if err != nil {
		t.Fatalf("failed to create the board: %v", err)
	}

	threadRepo := repository.NewThreadRepository(db)
	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID: board.ID, UserID: &authorID, Title: "はじめまして",
	})
	if err != nil {
		t.Fatalf("failed to create the thread: %v", err)
	}

	postRepo := repository.NewPostRepository(db)
	first, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID, UserID: &authorID, Number: 1, Body: "よろしくお願いします",
	})
	if err != nil {
		t.Fatalf("failed to create the first post: %v", err)
	}
	second, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID, UserID: &replierID, Number: 2, Body: ">>1 こちらこそ",
	})
	if err != nil {
		t.Fatalf("failed to create the second post: %v", err)
	}

	// The thread points back at its last post, which is the reference that makes
	// the order of the deletes matter rather than merely tidy.
	//
	// [Ja] スレッドは最終投稿を指し返します。削除の順序を、整っているかどうかではなく
	// 成否の問題にしているのがこの参照です。
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount: 2, LastPostID: second.ID, LastPostedAt: second.CreatedAt,
	}); err != nil {
		t.Fatalf("failed to update the last post of the thread: %v", err)
	}

	if _, err := repository.NewPostReferenceRepository(db).Create(ctx, repository.CreatePostReferenceInput{
		PostID: second.ID, ReferencedPostID: first.ID,
	}); err != nil {
		t.Fatalf("failed to create the post reference: %v", err)
	}

	for _, table := range cleanupTables {
		if countRows(t, db, table) == 0 {
			t.Fatalf("the table %s was left empty, so the cleanup of it would not be exercised", table)
		}
	}
}

// countRows returns how many rows the named table holds.
//
// [Ja] countRows は、指定した名前のテーブルが持つ行数を返します。
func countRows(t *testing.T, db *database.DB, table string) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(
		context.Background(), fmt.Sprintf("SELECT count(*) FROM %s", table),
	).Scan(&count); err != nil {
		t.Fatalf("failed to count the rows of %s: %v", table, err)
	}

	return count
}
