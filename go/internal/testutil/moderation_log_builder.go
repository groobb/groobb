package testutil

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// ModerationLogBuilder builds a moderation_logs row for tests via a fluent API.
// The action is required and has no default, since an entry exists to say which
// operation happened. Everything else starts absent: the operator and the three
// targets are nullable columns, and the reason defaults to the empty string the
// column itself defaults to.
//
// [Ja] ModerationLogBuilderはテスト用のmoderation_logs行をfluent APIで組み立てます。
// 操作の種類は必須で既定値はありません。記録は、どの操作が起きたかを述べるために存在する
// ためです。それ以外は何も無い状態から始まります。操作者と3つの対象はnullableな列であり、
// 理由の既定値は列自身の既定値と同じ空文字列です。
type ModerationLogBuilder struct {
	t            *testing.T
	db           *database.DB
	userID       *int64
	action       model.ModerationAction
	threadID     *int64
	postID       *int64
	targetUserID *int64
	reason       string
}

// NewModerationLogBuilder creates a ModerationLogBuilder for an entry no account
// is behind, which is what an operation performed by the operator looks like. A
// test whose subject is the administrator names them with WithUserID.
//
// [Ja] NewModerationLogBuilderは、どのアカウントも背後に持たない記録の
// ModerationLogBuilderを生成します。運用者として行われた操作の姿がこれです。管理者を
// 主題とするテストはWithUserIDでその人を名指します。
func NewModerationLogBuilder(t *testing.T, db *database.DB) *ModerationLogBuilder {
	t.Helper()
	return &ModerationLogBuilder{t: t, db: db}
}

// WithUserID sets the administrator who acted. Left unset, the entry is written
// with no operator, the way an operation performed outside any account is.
//
// [Ja] WithUserIDは操作した管理者を設定します。未設定なら、記録は操作者を持たない形で
// 書かれます。どのアカウントでもなく行われた操作がその形です。
func (b *ModerationLogBuilder) WithUserID(userID model.UserID) *ModerationLogBuilder {
	id := int64(userID)
	b.userID = &id
	return b
}

// WithAction sets the operation the entry records.
//
// [Ja] WithActionは記録する操作の種類を設定します。
func (b *ModerationLogBuilder) WithAction(action model.ModerationAction) *ModerationLogBuilder {
	b.action = action
	return b
}

// WithThreadID sets the thread the operation was aimed at.
//
// [Ja] WithThreadIDは操作の対象となったスレッドを設定します。
func (b *ModerationLogBuilder) WithThreadID(threadID model.ThreadID) *ModerationLogBuilder {
	id := int64(threadID)
	b.threadID = &id
	return b
}

// WithPostID sets the post the operation was aimed at. A post's unpublication
// names its thread as well, so a test building that entry sets both.
//
// [Ja] WithPostIDは操作の対象となった投稿を設定します。投稿の非公開は併せてその
// スレッドも名指すため、その記録を組み立てるテストは両方を設定します。
func (b *ModerationLogBuilder) WithPostID(postID model.PostID) *ModerationLogBuilder {
	id := int64(postID)
	b.postID = &id
	return b
}

// WithTargetUserID sets the account the operation was aimed at.
//
// [Ja] WithTargetUserIDは操作の対象となったアカウントを設定します。
func (b *ModerationLogBuilder) WithTargetUserID(targetUserID model.UserID) *ModerationLogBuilder {
	id := int64(targetUserID)
	b.targetUserID = &id
	return b
}

// WithReason sets what the administrator wrote about the operation. Left unset,
// the entry carries the empty string, which is what an operation given no reason
// holds.
//
// [Ja] WithReasonは管理者がその操作について書いたものを設定します。未設定なら記録は
// 空文字列を持ちます。理由を付けなかった操作が持つのがそれです。
func (b *ModerationLogBuilder) WithReason(reason string) *ModerationLogBuilder {
	b.reason = reason
	return b
}

// Build inserts the entry and returns its database-assigned ID, failing the test
// on error. id and the timestamps are left to the database defaults. It fails
// the test when no action has been set, since action is NOT NULL and an entry
// naming no operation says nothing.
//
// The action is written verbatim, including a value outside
// model.ModerationActions: the column enumerates nothing, so such a row is one
// the database accepts, and writing it directly is how a test reaches a history
// holding an operation this build does not know.
//
// [Ja] Buildは記録を挿入し、DBが採番したIDを返します。エラー時はテストを失敗させます。
// idとタイムスタンプはDBの既定値に任せます。actionはNOT NULLであり、どの操作も名指さない
// 記録は何も述べないため、未設定の場合はテストを失敗させます。
//
// actionはmodel.ModerationActionsの外の値も含めてそのまま書きます。列は値を列挙しないため
// そうした行はデータベースが受け入れる行であり、列を直接書くことが、このビルドの知らない
// 操作を持つ履歴へテストが辿り着く手立てです。
func (b *ModerationLogBuilder) Build() model.ModerationLogID {
	b.t.Helper()

	if b.action == "" {
		b.t.Fatal("ModerationLogBuilder には操作の種類が必要です (WithAction で設定してください)")
	}

	var id int64
	err := b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO moderation_logs (user_id, action, thread_id, post_id, target_user_id, reason)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		b.userID, string(b.action), b.threadID, b.postID, b.targetUserID, b.reason,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用操作履歴の作成に失敗: %v", err)
	}

	return model.ModerationLogID(id)
}
