package seed

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestCleanupTablesCoverTheSchemaは、スキーマの持つすべてのテーブルが2つの一覧の
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
				t.Errorf("テーブル %s が %s と %s の両方にある", table, previous, group.name)

				continue
			}
			classified[table] = group.name
		}
	}

	// SQLiteが自身のために持つテーブルは除外します。これらはsqlite_ で始まる名前を
	// 持ち (RiverのテーブルのAUTOINCREMENT列が連れてくるsqlite_sequenceなど)、シードが
	// 振り分けるものではありません。
	rows, err := db.Reader.QueryContext(
		context.Background(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
	)
	if err != nil {
		t.Fatalf("テーブルの一覧の取得に失敗: %v", err)
	}
	defer func() { _ = rows.Close() }()

	existing := make(map[string]bool, len(classified))
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("テーブル名の読み取りに失敗: %v", err)
		}
		existing[table] = true

		if _, exists := classified[table]; !exists {
			t.Errorf("テーブル %s がcleanupTablesにもpreservedTablesにも無い", table)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("テーブル名の走査に失敗: %v", err)
	}

	for table := range classified {
		if !existing[table] {
			t.Errorf("一覧にあるテーブル %s がスキーマに存在しない", table)
		}
	}
}

// TestCleanup_EmptiesTheTablesItManagesは、行の入ったデータベースがクリーンアップを
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
		t.Fatalf("トランザクションの開始に失敗: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := cleanup(ctx, tx); err != nil {
		t.Fatalf("cleanup()のエラー = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("トランザクションのコミットに失敗: %v", err)
	}

	for _, table := range cleanupTables {
		if count := countRows(t, db, table); count != 0 {
			t.Errorf("クリーンアップ後のテーブル %s の行数 = %d、期待値 = 0", table, count)
		}
	}

	// rolesは本テストが実際に行を入れられる保護対象テーブルであり、クリーンアップが
	// 保護対象へ手を出さないことを示せるのはこれだけです。他は本テストが書き込む手立てを
	// 持たない管理情報を保持します。
	//
	// 2行とも残る一方で、その一方を指すuser_rolesの行は空になります。これが、クリーン
	// アップが割り当てを消しても、割り当てていた対象までは消さないことを示します。2行とは、
	// マイグレーションが挿入する組み込みのadminロールと、本テストが書き込むロールです。
	// シードを実行して管理者を失ったインスタンスには、管理画面へ戻る手立てがありません。
	if count := countRows(t, db, "roles"); count != 2 {
		t.Errorf("クリーンアップ後のrolesテーブルの行数 = %d、期待値 = 2", count)
	}
	if slices.Contains(cleanupTables, "goose_db_version") {
		t.Error("goose_db_versionがcleanupTablesにある (空にするとデータベースが未マイグレーションに見える)")
	}
}

// populateForCleanupは、クリーンアップが管理する各テーブルへ1行ずつ書き込み、
// それらを削除の順序を決める外部キーで結び付けます。テストが書き込める保護対象テーブルにも
// 行を入れ、同じ実行でクリーンアップが何を残すのかも示せるようにします。
func populateForCleanup(t *testing.T, db *database.DB) {
	t.Helper()

	ctx := context.Background()

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Groobb"); err != nil {
		t.Fatalf("コミュニティの挿入に失敗: %v", err)
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
		t.Fatalf("ロールの挿入に失敗: %v", err)
	}
	if _, err := db.Writer.ExecContext(
		ctx, "INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", int64(authorID), roleID,
	); err != nil {
		t.Fatalf("ロールの割り当てに失敗: %v", err)
	}

	category, err := repository.NewCategoryRepository(db).Create(ctx, repository.CreateCategoryInput{
		Slug: "announcements", Name: "お知らせ", Position: 1,
	})
	if err != nil {
		t.Fatalf("カテゴリーの作成に失敗: %v", err)
	}

	board, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID, Slug: "general", Name: "雑談", Position: 1,
	})
	if err != nil {
		t.Fatalf("掲示板の作成に失敗: %v", err)
	}

	threadRepo := repository.NewThreadRepository(db)
	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID: board.ID, UserID: &authorID, Title: "はじめまして", Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("スレッドの作成に失敗: %v", err)
	}

	postRepo := repository.NewPostRepository(db)
	first, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID, UserID: &authorID, Number: 1, Body: "よろしくお願いします",
	})
	if err != nil {
		t.Fatalf("1件目の投稿の作成に失敗: %v", err)
	}
	second, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID, UserID: &replierID, Number: 2, Body: ">>1こちらこそ",
	})
	if err != nil {
		t.Fatalf("2件目の投稿の作成に失敗: %v", err)
	}

	// スレッドは最終投稿を指し返します。削除の順序を、整っているかどうかではなく
	// 成否の問題にしているのがこの参照です。
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount: 2, LastPostID: second.ID, LastPostedAt: second.CreatedAt,
	}); err != nil {
		t.Fatalf("スレッドの最終投稿の更新に失敗: %v", err)
	}

	if _, err := repository.NewPostReferenceRepository(db).Create(ctx, repository.CreatePostReferenceInput{
		PostID: second.ID, ReferencedPostID: first.ID,
	}); err != nil {
		t.Fatalf("レス参照の作成に失敗: %v", err)
	}

	// モデレーションの履歴は、このテーブルを所有するリポジトリがまだ無いため、
	// リポジトリではなく文で書き込みます。この行が投稿を名指すのは、周囲で空にされる
	// 中身を参照した状態にするためです。
	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO moderation_logs (user_id, action, thread_id, post_id, reason) VALUES (?, ?, ?, ?, ?)",
		int64(authorID), string(model.ModerationActionPostUnpublish), int64(thread.ID), int64(second.ID), "スパムのため",
	); err != nil {
		t.Fatalf("モデレーションログの挿入に失敗: %v", err)
	}

	for _, table := range cleanupTables {
		if countRows(t, db, table) == 0 {
			t.Fatalf("テーブル %s が空のままで、そのクリーンアップが検証されない", table)
		}
	}
}

// countRowsは、指定した名前のテーブルが持つ行数を返します。
func countRows(t *testing.T, db *database.DB, table string) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(
		context.Background(), fmt.Sprintf("SELECT count(*) FROM %s", table),
	).Scan(&count); err != nil {
		t.Fatalf("%s の行数の取得に失敗: %v", table, err)
	}

	return count
}
