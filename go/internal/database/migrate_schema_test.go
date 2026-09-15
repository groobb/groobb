package database_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
)

// TestMigrate_RestrictsCommunitiesToOneRowは、コミュニティを単一行に保つ制約が
// 2行目を拒否することを検証します。
func TestMigrate_RestrictsCommunitiesToOneRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	result, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Groobb")
	if err != nil {
		t.Fatalf("コミュニティの挿入に失敗: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("コミュニティのidの読み取りに失敗: %v", err)
	}
	if id != 1 {
		t.Errorf("最初のコミュニティのid = %d、期待値 = 1", id)
	}

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Other"); err == nil {
		t.Error("2つ目のコミュニティの挿入は失敗するはずだが、成功した")
	}
}

// TestMigrate_ListColumnsRequireJSONArraysは、リストを値に取る列が空配列と
// 文字列配列を受理し、配列以外の各JSON型を拒否することを検証します。
func TestMigrate_ListColumnsRequireJSONArrays(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	result, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	)
	if err != nil {
		t.Fatalf("ユーザーの挿入に失敗: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("ユーザーのidの読み取りに失敗: %v", err)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_two_factor_auths (user_id, secret) VALUES (?, ?)",
		userID, "secret",
	); err != nil {
		t.Fatalf("2段階認証の設定の挿入に失敗: %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO roles (name) VALUES (?)", "member"); err != nil {
		t.Fatalf("ロールの挿入に失敗: %v", err)
	}

	var recoveryCodes string
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT recovery_codes FROM user_two_factor_auths WHERE user_id = ?",
		userID,
	).Scan(&recoveryCodes); err != nil {
		t.Fatalf("既定のリカバリーコードの読み取りに失敗: %v", err)
	}
	if recoveryCodes != "[]" {
		t.Errorf("既定のrecovery_codes = %q、期待値 = []", recoveryCodes)
	}

	var scopes string
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT scopes FROM roles WHERE name = ?",
		"member",
	).Scan(&scopes); err != nil {
		t.Fatalf("既定のscopesの読み取りに失敗: %v", err)
	}
	if scopes != "[]" {
		t.Errorf("既定のscopes = %q、期待値 = []", scopes)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"UPDATE user_two_factor_auths SET recovery_codes = ? WHERE user_id = ?",
		"[\"recovery-code\"]", userID,
	); err != nil {
		t.Fatalf("recovery_codesを文字列配列へ更新するのに失敗: %v", err)
	}
	if _, err := db.Writer.ExecContext(
		ctx,
		"UPDATE roles SET scopes = ? WHERE name = ?",
		"[\"read\"]", "member",
	); err != nil {
		t.Fatalf("scopesを文字列配列へ更新するのに失敗: %v", err)
	}

	invalidValues := []struct {
		name  string
		value string
	}{
		{name: "オブジェクト", value: "{}"},
		{name: "null", value: "null"},
		{name: "数値", value: "1"},
		{name: "文字列", value: "\"scope\""},
	}

	for _, tt := range invalidValues {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Writer.ExecContext(
				ctx,
				"UPDATE user_two_factor_auths SET recovery_codes = ? WHERE user_id = ?",
				tt.value, userID,
			); err == nil {
				t.Errorf("recovery_codesを %s にするのは失敗するはずだが、成功した", tt.value)
			}

			if _, err := db.Writer.ExecContext(
				ctx,
				"UPDATE roles SET scopes = ? WHERE name = ?",
				tt.value, "member",
			); err == nil {
				t.Errorf("scopesを %s にするのは失敗するはずだが、成功した", tt.value)
			}
		})
	}
}

// TestMigrate_EnforcesUserPasswordForeignKeyは、代表的なusersの子テーブルが
// 孤立行を拒否し、親行の削除を子行へカスケードすることを検証します。
func TestMigrate_EnforcesUserPasswordForeignKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_passwords (user_id, password_digest) VALUES (?, ?)",
		999, "digest",
	); err == nil {
		t.Error("存在しないユーザーのパスワードの挿入は失敗するはずだが、成功した")
	}

	result, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	)
	if err != nil {
		t.Fatalf("ユーザーの挿入に失敗: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("ユーザーのidの読み取りに失敗: %v", err)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_passwords (user_id, password_digest) VALUES (?, ?)",
		userID, "digest",
	); err != nil {
		t.Fatalf("パスワードの挿入に失敗: %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID); err != nil {
		t.Fatalf("ユーザーの削除に失敗: %v", err)
	}

	var passwordCount int
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM user_passwords WHERE user_id = ?",
		userID,
	).Scan(&passwordCount); err != nil {
		t.Fatalf("ユーザーの削除後のパスワードの計数に失敗: %v", err)
	}
	if passwordCount != 0 {
		t.Errorf("ユーザーの削除後のパスワードの数 = %d、期待値 = 0", passwordCount)
	}
}

// communityContentIDsは、コミュニティの中身の各テーブルの行1つずつのidです。
// カテゴリーが掲示板を、掲示板がスレッドを、スレッドが投稿1件を持つ形にしています。
type communityContentIDs struct {
	userID     int64
	categoryID int64
	boardID    int64
	threadID   int64
	postID     int64
}

// insertCommunityContentはカテゴリー・掲示板・スレッド・投稿を1つずつと、
// それらを書いたユーザーを投入し、そのidを返します。
func insertCommunityContent(t *testing.T, db *database.DB) communityContentIDs {
	t.Helper()

	ctx := context.Background()
	ids := communityContentIDs{}

	ids.userID = insertRow(t, db,
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	)
	ids.categoryID = insertRow(t, db,
		"INSERT INTO categories (slug, name) VALUES (?, ?)",
		"general", "全般",
	)
	ids.boardID = insertRow(t, db,
		"INSERT INTO boards (category_id, slug, name) VALUES (?, ?, ?)",
		ids.categoryID, "chat", "雑談",
	)
	ids.threadID = insertRow(t, db,
		"INSERT INTO threads (board_id, user_id, title, language) VALUES (?, ?, ?, ?)",
		ids.boardID, ids.userID, "はじめてのスレッド", "ja",
	)
	ids.postID = insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 1, "こんにちは",
	)

	if _, err := db.Writer.ExecContext(
		ctx,
		"UPDATE threads SET posts_count = 1, last_post_id = ? WHERE id = ?",
		ids.postID, ids.threadID,
	); err != nil {
		t.Fatalf("スレッドを最後の投稿へ向けるのに失敗: %v", err)
	}

	return ids
}

func insertRow(t *testing.T, db *database.DB, statement string, args ...any) int64 {
	t.Helper()

	result, err := db.Writer.ExecContext(context.Background(), statement, args...)
	if err != nil {
		t.Fatalf("%q の実行に失敗: %v", statement, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("%q が挿入したidの読み取りに失敗: %v", statement, err)
	}

	return id
}

func countRows(t *testing.T, db *database.DB, query string, args ...any) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("%q による行の計数に失敗: %v", query, err)
	}

	return count
}

// TestMigrate_SlugUniquenessIgnoresLetterCaseは、slug列の照合順序により
// 大文字小文字だけが異なる値が衝突することを検証します。これにより1つのslugが
// 2つのカテゴリーや2つの掲示板を指すことはありません。
func TestMigrate_SlugUniquenessIgnoresLetterCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		statement string
		args      []any
	}{
		{
			name:      "カテゴリー",
			statement: "INSERT INTO categories (slug, name) VALUES (?, ?)",
			args:      []any{"GENERAL", "別のカテゴリー"},
		},
		{
			name:      "掲示板",
			statement: "INSERT INTO boards (category_id, slug, name) VALUES ((SELECT id FROM categories), ?, ?)",
			args:      []any{"CHAT", "別の掲示板"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := migratedTestDB(t)
			insertCommunityContent(t, db)

			if _, err := db.Writer.ExecContext(context.Background(), tt.statement, tt.args...); err == nil {
				t.Errorf("大文字小文字だけが異なる同じ %s のslugの挿入は失敗するはずだが、成功した", tt.name)
			}
		})
	}
}

// TestMigrate_KeepsPostNumbersUniqueWithinAThreadは、レス番号がスレッド内の
// ちょうど1つの投稿を指すことを検証します。これが >>Nとアンカー #p{number} が投稿を
// 永久に指せる根拠であり、同じ番号は別のスレッドで改めて使えます。
func TestMigrate_KeepsPostNumbersUniqueWithinAThread(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 1, "番号が重複した投稿",
	); err == nil {
		t.Error("同じスレッド内でのレス番号の再利用は失敗するはずだが、成功した")
	}

	otherThreadID := insertRow(t, db,
		"INSERT INTO threads (board_id, user_id, title, language) VALUES (?, ?, ?, ?)",
		ids.boardID, ids.userID, "別のスレッド", "ja",
	)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		otherThreadID, ids.userID, 1, "別スレッドの1件目",
	); err != nil {
		t.Errorf("別のスレッドでの同じレス番号は受理されるはずだが、失敗した: %v", err)
	}
}

// TestMigrate_RecordsEachPostReferenceOnceは、投稿の組を記録できるのが1度だけで
// あることを検証します。>>1を2度書いた本文が、それが表している1つの関係を2行に
// することはなく、同じ投稿が別の投稿に対して行った参照はそれ自体で1つの関係として
// 残ります。
func TestMigrate_RecordsEachPostReferenceOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	replyID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, ">>1 >>1こんにちは",
	)
	insertRow(t, db,
		"INSERT INTO post_references (post_id, referenced_post_id) VALUES (?, ?)",
		replyID, ids.postID,
	)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO post_references (post_id, referenced_post_id) VALUES (?, ?)",
		replyID, ids.postID,
	); err == nil {
		t.Error("同じ参照を2度記録するのは失敗するはずだが、成功した")
	}

	otherPostID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 3, "もう1つの投稿",
	)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO post_references (post_id, referenced_post_id) VALUES (?, ?)",
		replyID, otherPostID,
	); err != nil {
		t.Errorf("同じ投稿から別の投稿への参照は受理されるはずだが、失敗した: %v", err)
	}
}

// TestMigrate_UncategorizesTheBoardsOfADeletedCategoryは、カテゴリーの削除が、
// それが並べていた掲示板を巻き込むことも運営が移し先を決めるのを待つこともせず、
// カテゴリーを持たないまま残すことを検証します。どのカテゴリーにも属さない掲示板は正常な
// 状態であり (ADR 0011)、削除はそこへ置きます。
func TestMigrate_UncategorizesTheBoardsOfADeletedCategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM categories WHERE id = ?", ids.categoryID); err != nil {
		t.Fatalf("掲示板を持つカテゴリーの削除は成功するはずだが、失敗した: %v", err)
	}

	var categoryID *int64
	if err := db.Reader.QueryRowContext(ctx, "SELECT category_id FROM boards WHERE id = ?", ids.boardID).Scan(&categoryID); err != nil {
		t.Fatalf("掲示板の読み戻しに失敗: %v", err)
	}
	if categoryID != nil {
		t.Errorf("カテゴリーの削除後の掲示板のcategory_id = %d、期待値 = NULL", *categoryID)
	}
}

// TestMigrate_CascadesDeletionThroughBoardContentsは、掲示板の削除が配下の
// スレッド・投稿・参照まで及ぶことを検証します。掲示板が無くなった時点で、それらは
// いずれも独立した意味を持たないためです。
func TestMigrate_CascadesDeletionThroughBoardContents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	replyID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, ">>1こんにちは",
	)
	insertRow(t, db,
		"INSERT INTO post_references (post_id, referenced_post_id) VALUES (?, ?)",
		replyID, ids.postID,
	)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM boards WHERE id = ?", ids.boardID); err != nil {
		t.Fatalf("掲示板の削除に失敗: %v", err)
	}

	tables := []string{"threads", "posts", "post_references"}
	for _, table := range tables {
		if count := countRows(t, db, "SELECT COUNT(*) FROM "+table); count != 0 {
			t.Errorf("掲示板の削除後の %s の行数 = %d、期待値 = 0", table, count)
		}
	}
}

// TestMigrate_CascadesDeletionFromEitherEndOfAPostReferenceは、2本の外部キーを
// 個別に検証します。参照元と参照先のどちらの投稿を削除しても、反対側の投稿を残したまま
// 参照関係だけが削除されます。
func TestMigrate_CascadesDeletionFromEitherEndOfAPostReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		deleteSource bool
	}{
		{name: "参照元の投稿", deleteSource: true},
		{name: "参照先の投稿", deleteSource: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := migratedTestDB(t)
			ids := insertCommunityContent(t, db)
			replyID := insertRow(t, db,
				"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
				ids.threadID, ids.userID, 2, ">>1こんにちは",
			)
			insertRow(t, db,
				"INSERT INTO post_references (post_id, referenced_post_id) VALUES (?, ?)",
				replyID, ids.postID,
			)

			deletedPostID := ids.postID
			survivingPostID := replyID
			if tt.deleteSource {
				deletedPostID = replyID
				survivingPostID = ids.postID
			}

			if _, err := db.Writer.ExecContext(ctx, "DELETE FROM posts WHERE id = ?", deletedPostID); err != nil {
				t.Fatalf("投稿の削除に失敗: %v", err)
			}

			if count := countRows(t, db, "SELECT COUNT(*) FROM post_references"); count != 0 {
				t.Errorf("%s を削除した後の投稿の参照の数 = %d、期待値 = 0", tt.name, count)
			}
			if count := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ?", survivingPostID); count != 1 {
				t.Errorf("%s を削除した後に残る投稿の数 = %d、期待値 = 1", tt.name, count)
			}
		})
	}
}

// TestMigrate_KeepsPostsOfADeletedUserは、ユーザーの削除が、そのユーザーの
// 書いたスレッドや投稿を消すのではなく作者だけを外すことを検証します。これにより、
// 他人の返信が返信先の会話を失うことがありません。
func TestMigrate_KeepsPostsOfADeletedUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = ?", ids.userID); err != nil {
		t.Fatalf("ユーザーの削除に失敗: %v", err)
	}

	orphanedThreads := countRows(t, db, "SELECT COUNT(*) FROM threads WHERE id = ? AND user_id IS NULL", ids.threadID)
	if orphanedThreads != 1 {
		t.Errorf("作者を持たないまま残ったスレッドの数 = %d、期待値 = 1", orphanedThreads)
	}

	orphanedPosts := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ? AND user_id IS NULL", ids.postID)
	if orphanedPosts != 1 {
		t.Errorf("作者を持たないまま残った投稿の数 = %d、期待値 = 1", orphanedPosts)
	}
}

// TestMigrate_ClearsTheLastPostOfAThreadWhenItIsDeletedは、スレッドが指している
// 投稿を削除したときに、スレッドごと消えるのではなくスレッドが残り、その参照だけが
// 外れることを検証します。
func TestMigrate_ClearsTheLastPostOfAThreadWhenItIsDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM posts WHERE id = ?", ids.postID); err != nil {
		t.Fatalf("最後の投稿の削除に失敗: %v", err)
	}

	remaining := countRows(t, db, "SELECT COUNT(*) FROM threads WHERE id = ? AND last_post_id IS NULL", ids.threadID)
	if remaining != 1 {
		t.Errorf("最後の投稿が外れて残ったスレッドの数 = %d、期待値 = 1", remaining)
	}
}
