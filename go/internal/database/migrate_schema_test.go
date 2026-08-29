package database_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
)

// TestMigrate_RestrictsCommunitiesToOneRow verifies that the singleton
// community constraint rejects a second row.
//
// [Ja] TestMigrate_RestrictsCommunitiesToOneRow は、コミュニティを単一行に保つ制約が
// 2 行目を拒否することを検証します。
func TestMigrate_RestrictsCommunitiesToOneRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	result, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Groobb")
	if err != nil {
		t.Fatalf("failed to insert the community: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read the community id: %v", err)
	}
	if id != 1 {
		t.Errorf("the first community id = %d, want 1", id)
	}

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Other"); err == nil {
		t.Error("inserting a second community should fail, but it succeeded")
	}
}

// TestMigrate_ListColumnsRequireJSONArrays verifies that list-valued columns
// accept empty and string arrays while rejecting every non-array JSON type.
//
// [Ja] TestMigrate_ListColumnsRequireJSONArrays は、リストを値に取る列が空配列と
// 文字列配列を受理し、配列以外の各 JSON 型を拒否することを検証します。
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
		t.Fatalf("failed to insert a user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read the user id: %v", err)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_two_factor_auths (user_id, secret) VALUES (?, ?)",
		userID, "secret",
	); err != nil {
		t.Fatalf("failed to insert two-factor authentication settings: %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO roles (name) VALUES (?)", "member"); err != nil {
		t.Fatalf("failed to insert a role: %v", err)
	}

	var recoveryCodes string
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT recovery_codes FROM user_two_factor_auths WHERE user_id = ?",
		userID,
	).Scan(&recoveryCodes); err != nil {
		t.Fatalf("failed to read the default recovery codes: %v", err)
	}
	if recoveryCodes != "[]" {
		t.Errorf("default recovery_codes = %q, want []", recoveryCodes)
	}

	var scopes string
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT scopes FROM roles WHERE name = ?",
		"member",
	).Scan(&scopes); err != nil {
		t.Fatalf("failed to read the default scopes: %v", err)
	}
	if scopes != "[]" {
		t.Errorf("default scopes = %q, want []", scopes)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"UPDATE user_two_factor_auths SET recovery_codes = ? WHERE user_id = ?",
		"[\"recovery-code\"]", userID,
	); err != nil {
		t.Fatalf("updating recovery_codes to a string array failed: %v", err)
	}
	if _, err := db.Writer.ExecContext(
		ctx,
		"UPDATE roles SET scopes = ? WHERE name = ?",
		"[\"read\"]", "member",
	); err != nil {
		t.Fatalf("updating scopes to a string array failed: %v", err)
	}

	invalidValues := []struct {
		name  string
		value string
	}{
		{name: "object", value: "{}"},
		{name: "null", value: "null"},
		{name: "number", value: "1"},
		{name: "string", value: "\"scope\""},
	}

	for _, tt := range invalidValues {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Writer.ExecContext(
				ctx,
				"UPDATE user_two_factor_auths SET recovery_codes = ? WHERE user_id = ?",
				tt.value, userID,
			); err == nil {
				t.Errorf("setting recovery_codes to %s should fail, but it succeeded", tt.value)
			}

			if _, err := db.Writer.ExecContext(
				ctx,
				"UPDATE roles SET scopes = ? WHERE name = ?",
				tt.value, "member",
			); err == nil {
				t.Errorf("setting scopes to %s should fail, but it succeeded", tt.value)
			}
		})
	}
}

// TestMigrate_EnforcesUserPasswordForeignKey verifies that a representative
// user child table rejects orphan rows and cascades deletion of its parent.
//
// [Ja] TestMigrate_EnforcesUserPasswordForeignKey は、代表的な users の子テーブルが
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
		t.Error("inserting a password for a missing user should fail, but it succeeded")
	}

	result, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	)
	if err != nil {
		t.Fatalf("failed to insert a user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read the user id: %v", err)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_passwords (user_id, password_digest) VALUES (?, ?)",
		userID, "digest",
	); err != nil {
		t.Fatalf("failed to insert a password: %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID); err != nil {
		t.Fatalf("failed to delete the user: %v", err)
	}

	var passwordCount int
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM user_passwords WHERE user_id = ?",
		userID,
	).Scan(&passwordCount); err != nil {
		t.Fatalf("failed to count passwords after deleting the user: %v", err)
	}
	if passwordCount != 0 {
		t.Errorf("password count after deleting the user = %d, want 0", passwordCount)
	}
}

// communityContentIDs are the ids of one row per community content table, laid
// out as a category holding a board, a board holding a thread, and a thread
// holding one post.
//
// [Ja] communityContentIDs は、コミュニティの中身の各テーブルの行 1 つずつの id です。
// カテゴリーが掲示板を、掲示板がスレッドを、スレッドが投稿 1 件を持つ形にしています。
type communityContentIDs struct {
	userID     int64
	categoryID int64
	boardID    int64
	threadID   int64
	postID     int64
}

// insertCommunityContent inserts one category, board, thread and post, plus the
// user who wrote them, and returns their ids.
//
// [Ja] insertCommunityContent はカテゴリー・掲示板・スレッド・投稿を 1 つずつと、
// それらを書いたユーザーを投入し、その id を返します。
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
		t.Fatalf("failed to point the thread at its last post: %v", err)
	}

	return ids
}

func insertRow(t *testing.T, db *database.DB, statement string, args ...any) int64 {
	t.Helper()

	result, err := db.Writer.ExecContext(context.Background(), statement, args...)
	if err != nil {
		t.Fatalf("failed to run %q: %v", statement, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read the id inserted by %q: %v", statement, err)
	}

	return id
}

func countRows(t *testing.T, db *database.DB, query string, args ...any) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("failed to count rows with %q: %v", query, err)
	}

	return count
}

// TestMigrate_SlugUniquenessIgnoresLetterCase verifies that the collation on
// the slug columns makes values that differ only in case collide, so that one
// slug cannot address two categories or two boards.
//
// [Ja] TestMigrate_SlugUniquenessIgnoresLetterCase は、slug 列の照合順序により
// 大文字小文字だけが異なる値が衝突することを検証します。これにより 1 つの slug が
// 2 つのカテゴリーや 2 つの掲示板を指すことはありません。
func TestMigrate_SlugUniquenessIgnoresLetterCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		statement string
		args      []any
	}{
		{
			name:      "category",
			statement: "INSERT INTO categories (slug, name) VALUES (?, ?)",
			args:      []any{"GENERAL", "別のカテゴリー"},
		},
		{
			name:      "board",
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
				t.Errorf("inserting the same %s slug in a different case should fail, but it succeeded", tt.name)
			}
		})
	}
}

// TestMigrate_KeepsPostNumbersUniqueWithinAThread verifies that a reply number
// identifies exactly one post in its thread, which is what lets >>N and the
// #p{number} anchor address a post permanently, while the same number is free
// to be used again in another thread.
//
// [Ja] TestMigrate_KeepsPostNumbersUniqueWithinAThread は、レス番号がスレッド内の
// ちょうど 1 つの投稿を指すことを検証します。これが >>N とアンカー #p{number} が投稿を
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
		t.Error("reusing a reply number within the same thread should fail, but it succeeded")
	}

	otherThreadID := insertRow(t, db,
		"INSERT INTO threads (board_id, user_id, title, language) VALUES (?, ?, ?, ?)",
		ids.boardID, ids.userID, "別のスレッド", "ja",
	)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		otherThreadID, ids.userID, 1, "別スレッドの 1 件目",
	); err != nil {
		t.Errorf("the same reply number in another thread should be accepted, but it failed: %v", err)
	}
}

// TestMigrate_RecordsEachPostReferenceOnce verifies that a pair of posts can be
// recorded only once, so a body writing >>1 twice cannot turn the one
// relationship it expresses into two rows, while a reference the same post
// makes to a different post stays a relationship of its own.
//
// [Ja] TestMigrate_RecordsEachPostReferenceOnce は、投稿の組を記録できるのが 1 度だけで
// あることを検証します。>>1 を 2 度書いた本文が、それが表している 1 つの関係を 2 行に
// することはなく、同じ投稿が別の投稿に対して行った参照はそれ自体で 1 つの関係として
// 残ります。
func TestMigrate_RecordsEachPostReferenceOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	replyID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, ">>1 >>1 こんにちは",
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
		t.Error("recording the same reference twice should fail, but it succeeded")
	}

	otherPostID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 3, "もう 1 つの投稿",
	)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO post_references (post_id, referenced_post_id) VALUES (?, ?)",
		replyID, otherPostID,
	); err != nil {
		t.Errorf("referring to another post from the same post should be accepted, but it failed: %v", err)
	}
}

// TestMigrate_UncategorizesTheBoardsOfADeletedCategory verifies that deleting a
// category leaves the boards it listed behind without one, rather than taking
// them with it or waiting for an operator to move them somewhere first. A board
// outside every category is a normal state (ADR 0011), so that is where the
// delete puts them.
//
// [Ja] TestMigrate_UncategorizesTheBoardsOfADeletedCategory は、カテゴリーの削除が、
// それが並べていた掲示板を巻き込むことも運営が移し先を決めるのを待つこともせず、
// カテゴリーを持たないまま残すことを検証します。どのカテゴリーにも属さない掲示板は正常な
// 状態であり (ADR 0011)、削除はそこへ置きます。
func TestMigrate_UncategorizesTheBoardsOfADeletedCategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM categories WHERE id = ?", ids.categoryID); err != nil {
		t.Fatalf("deleting a category that still holds a board should succeed, but it failed: %v", err)
	}

	var categoryID *int64
	if err := db.Reader.QueryRowContext(ctx, "SELECT category_id FROM boards WHERE id = ?", ids.boardID).Scan(&categoryID); err != nil {
		t.Fatalf("failed to read the board back: %v", err)
	}
	if categoryID != nil {
		t.Errorf("the board's category_id after deleting its category = %d, want NULL", *categoryID)
	}
}

// TestMigrate_CascadesDeletionThroughBoardContents verifies that deleting a
// board takes the threads, posts and references below it with it, since none of
// them carry a meaning of their own once the board is gone.
//
// [Ja] TestMigrate_CascadesDeletionThroughBoardContents は、掲示板の削除が配下の
// スレッド・投稿・参照まで及ぶことを検証します。掲示板が無くなった時点で、それらは
// いずれも独立した意味を持たないためです。
func TestMigrate_CascadesDeletionThroughBoardContents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	replyID := insertRow(t, db,
		"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
		ids.threadID, ids.userID, 2, ">>1 こんにちは",
	)
	insertRow(t, db,
		"INSERT INTO post_references (post_id, referenced_post_id) VALUES (?, ?)",
		replyID, ids.postID,
	)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM boards WHERE id = ?", ids.boardID); err != nil {
		t.Fatalf("failed to delete the board: %v", err)
	}

	tables := []string{"threads", "posts", "post_references"}
	for _, table := range tables {
		if count := countRows(t, db, "SELECT COUNT(*) FROM "+table); count != 0 {
			t.Errorf("%s row count after deleting the board = %d, want 0", table, count)
		}
	}
}

// TestMigrate_CascadesDeletionFromEitherEndOfAPostReference verifies both
// foreign keys independently: deleting either the source or the referenced
// post removes the relationship without removing the post at its other end.
//
// [Ja] TestMigrate_CascadesDeletionFromEitherEndOfAPostReference は、2 本の外部キーを
// 個別に検証します。参照元と参照先のどちらの投稿を削除しても、反対側の投稿を残したまま
// 参照関係だけが削除されます。
func TestMigrate_CascadesDeletionFromEitherEndOfAPostReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		deleteSource bool
	}{
		{name: "source post", deleteSource: true},
		{name: "referenced post", deleteSource: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := migratedTestDB(t)
			ids := insertCommunityContent(t, db)
			replyID := insertRow(t, db,
				"INSERT INTO posts (thread_id, user_id, number, body) VALUES (?, ?, ?, ?)",
				ids.threadID, ids.userID, 2, ">>1 こんにちは",
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
				t.Fatalf("failed to delete the post: %v", err)
			}

			if count := countRows(t, db, "SELECT COUNT(*) FROM post_references"); count != 0 {
				t.Errorf("post reference count after deleting the %s = %d, want 0", tt.name, count)
			}
			if count := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ?", survivingPostID); count != 1 {
				t.Errorf("surviving post count after deleting the %s = %d, want 1", tt.name, count)
			}
		})
	}
}

// TestMigrate_KeepsPostsOfADeletedUser verifies that removing a user detaches
// the author from the threads and posts they wrote instead of deleting them,
// which is what keeps other people's replies from losing the conversation they
// answer.
//
// [Ja] TestMigrate_KeepsPostsOfADeletedUser は、ユーザーの削除が、そのユーザーの
// 書いたスレッドや投稿を消すのではなく作者だけを外すことを検証します。これにより、
// 他人の返信が返信先の会話を失うことがありません。
func TestMigrate_KeepsPostsOfADeletedUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = ?", ids.userID); err != nil {
		t.Fatalf("failed to delete the user: %v", err)
	}

	orphanedThreads := countRows(t, db, "SELECT COUNT(*) FROM threads WHERE id = ? AND user_id IS NULL", ids.threadID)
	if orphanedThreads != 1 {
		t.Errorf("threads left without an author = %d, want 1", orphanedThreads)
	}

	orphanedPosts := countRows(t, db, "SELECT COUNT(*) FROM posts WHERE id = ? AND user_id IS NULL", ids.postID)
	if orphanedPosts != 1 {
		t.Errorf("posts left without an author = %d, want 1", orphanedPosts)
	}
}

// TestMigrate_ClearsTheLastPostOfAThreadWhenItIsDeleted verifies that deleting
// the post a thread points at leaves the thread in place with the pointer
// cleared, rather than taking the thread with it.
//
// [Ja] TestMigrate_ClearsTheLastPostOfAThreadWhenItIsDeleted は、スレッドが指している
// 投稿を削除したときに、スレッドごと消えるのではなくスレッドが残り、その参照だけが
// 外れることを検証します。
func TestMigrate_ClearsTheLastPostOfAThreadWhenItIsDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)
	ids := insertCommunityContent(t, db)

	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM posts WHERE id = ?", ids.postID); err != nil {
		t.Fatalf("failed to delete the last post: %v", err)
	}

	remaining := countRows(t, db, "SELECT COUNT(*) FROM threads WHERE id = ? AND last_post_id IS NULL", ids.threadID)
	if remaining != 1 {
		t.Errorf("threads surviving with a cleared last post = %d, want 1", remaining)
	}
}
