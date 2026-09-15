package usecase_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// findPostは、投稿を名指す組で投稿を読み戻します。どの状態で残されたかを問う検証の
// ためのものです。
func findPost(t *testing.T, db *database.DB, threadID model.ThreadID, number int) *model.Post {
	t.Helper()

	post, err := repository.NewPostRepository(db).FindByThreadIDAndNumber(context.Background(), threadID, number)
	if err != nil {
		t.Fatalf("FindByThreadIDAndNumber()のエラー = %v", err)
	}
	if post == nil {
		t.Fatalf("投稿を引けない: thread_id=%v number=%d", threadID, number)
	}
	return post
}

// TestUnpublishPostUsecase_Execute_Successは、管理者が投稿1件の本文を視界から
// 外せること、投稿がレス番号と本文を保つこと、その周りのスレッドが触れられないこと、
// そして履歴がスレッドと投稿の両方を名指してその操作を運ぶことを検証します。
func TestUnpublishPostUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	before := findPost(t, f.db, f.thread.ID, 1)

	if err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Number:   1,
		Reason:   "  個人情報が書かれていたため  ",
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	post := findPost(t, f.db, f.thread.ID, 1)
	if post.UnpublishedAt == nil {
		t.Fatal("非公開後のUnpublishedAt = nil、期待値 = 非nil")
	}
	if post.Number != before.Number {
		t.Errorf("Number = %d、期待値 = %d", post.Number, before.Number)
	}
	if post.Body != before.Body {
		t.Errorf("Body = %q、期待値 = %q", post.Body, before.Body)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != f.thread.PostsCount {
		t.Errorf("PostsCount = %d、期待値 = %d", thread.PostsCount, f.thread.PostsCount)
	}
	if thread.LastPostID == nil || *thread.LastPostID != *f.thread.LastPostID {
		t.Errorf("LastPostID = %v、期待値 = %v", thread.LastPostID, f.thread.LastPostID)
	}
	if !thread.LastPostedAt.Equal(f.thread.LastPostedAt) {
		t.Errorf("LastPostedAt = %v、期待値 = %v", thread.LastPostedAt, f.thread.LastPostedAt)
	}
	if thread.UnpublishedAt != nil {
		t.Errorf("スレッドのUnpublishedAt = %v、期待値 = nil", thread.UnpublishedAt)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 1 {
		t.Fatalf("操作履歴の件数 = %d、期待値 = 1", len(logs))
	}
	log := logs[0]
	if log.Action != model.ModerationActionPostUnpublish {
		t.Errorf("Action = %q、期待値 = %q", log.Action, model.ModerationActionPostUnpublish)
	}
	if log.UserID == nil || *log.UserID != f.admin {
		t.Errorf("UserID = %v、期待値 = %s", log.UserID, f.admin)
	}
	if log.ThreadID == nil || *log.ThreadID != f.thread.ID {
		t.Errorf("ThreadID = %v、期待値 = %s", log.ThreadID, f.thread.ID)
	}
	if log.PostID == nil || *log.PostID != post.ID {
		t.Errorf("PostID = %v、期待値 = %s", log.PostID, post.ID)
	}
	if log.Reason != "個人情報が書かれていたため" {
		t.Errorf("Reason = %q、期待値 = %q", log.Reason, "個人情報が書かれていたため")
	}
	if log.TargetUserID != nil {
		t.Errorf("TargetUserID = %v、期待値 = nil", log.TargetUserID)
	}
}

// TestUnpublishPostUsecase_Execute_ScopedActorは、この操作を許すのが
// post_unpublication:writeであること、そして別のモデレーションのスコープだけを持つ
// ロールが拒否されることを検証します。
func TestUnpublishPostUsecase_Execute_ScopedActor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roleName     model.RoleName
		scopes       []model.Scope
		wantAdmitted bool
	}{
		{
			name:         "post_unpublication:writeを持つロールは許される",
			roleName:     "post_unpublisher",
			scopes:       []model.Scope{model.ScopePostUnpublicationWrite},
			wantAdmitted: true,
		},
		{
			name:     "スレッドの非公開のスコープだけでは拒否される",
			roleName: "thread_unpublisher",
			scopes:   []model.Scope{model.ScopeThreadUnpublicationWrite},
		},
		{
			name:     "スコープを持たないロールは拒否される",
			roleName: "no_scope",
			scopes:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, ctx := newThreadModerationUsecases(t)
			actorID := seedScopedActor(t, f.db, tt.roleName, tt.scopes)

			err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
				Actor:    usecase.UserActor(actorID),
				ThreadID: f.thread.ID,
				Number:   1,
			})

			if tt.wantAdmitted {
				if err != nil {
					t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
				}
				if findPost(t, f.db, f.thread.ID, 1).UnpublishedAt == nil {
					t.Error("非公開後のUnpublishedAt = nil、期待値 = 非nil")
				}
				return
			}

			assertAppErrCode(t, err, model.AppErrCodeForbidden)
			if findPost(t, f.db, f.thread.ID, 1).UnpublishedAt != nil {
				t.Error("拒否後のUnpublishedAt = 非nil、期待値 = nil")
			}
			if logs := listModerationLogs(t, f.db); len(logs) != 0 {
				t.Errorf("拒否後の操作履歴の件数 = %d、期待値 = 0", len(logs))
			}
		})
	}
}

// TestUnpublishPostUsecase_Execute_UnknownTargetは、コミュニティが持たないスレッドと、
// スレッドが発行していないレス番号が、どちらもリソースの不在として答えられることを検証
// します。
func TestUnpublishPostUsecase_Execute_UnknownTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		threadGap model.ThreadID
		number    int
	}{
		{name: "コミュニティが持たないスレッド", threadGap: 1000, number: 1},
		{name: "スレッドが発行していないレス番号", number: 2},
		{name: "レス番号として使われない0", number: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, ctx := newThreadModerationUsecases(t)

			err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
				Actor:    usecase.UserActor(f.admin),
				ThreadID: f.thread.ID + tt.threadGap,
				Number:   tt.number,
			})

			assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
			if logs := listModerationLogs(t, f.db); len(logs) != 0 {
				t.Errorf("操作履歴の件数 = %d、期待値 = 0", len(logs))
			}
		})
	}
}

// TestUnpublishPostUsecase_Execute_UnpublishedThreadは、スレッドが非公開の投稿が、
// スレッド自身のコードで拒否されることを検証します。スレッドが丸ごと視界の外にあり、この
// 操作が取り除くべき本文はコミュニティの中に立っていないためです。
func TestUnpublishPostUsecase_Execute_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	unpublishThread(t, f.db, f.thread.ID)

	err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Number:   1,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceUnpublished)
	if findPost(t, f.db, f.thread.ID, 1).UnpublishedAt != nil {
		t.Error("拒否後のUnpublishedAt = 非nil、期待値 = nil")
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d、期待値 = 0", len(logs))
	}
}

// TestUnpublishPostUsecase_Execute_AlreadyUnpublishedは、既に非公開の投稿の非公開が、
// 履歴に2件目を足さず、視界から外された時刻も動かさずに成功することを検証します。
func TestUnpublishPostUsecase_Execute_AlreadyUnpublished(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	input := usecase.UnpublishPostInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Number:   1,
	}

	if err := f.unpublishPostUC.Execute(ctx, input); err != nil {
		t.Fatalf("1度目のExecute()のエラー = %v、期待値 = nil", err)
	}
	unpublishedAt := findPost(t, f.db, f.thread.ID, 1).UnpublishedAt

	if err := f.unpublishPostUC.Execute(ctx, input); err != nil {
		t.Fatalf("2度目のExecute()のエラー = %v、期待値 = nil", err)
	}

	if got := findPost(t, f.db, f.thread.ID, 1).UnpublishedAt; got == nil || !got.Equal(*unpublishedAt) {
		t.Errorf("2度目の後のUnpublishedAt = %v、期待値 = %v", got, unpublishedAt)
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 1 {
		t.Errorf("操作履歴の件数 = %d、期待値 = 1", len(logs))
	}
}

// TestUnpublishPostUsecase_Execute_InvalidReasonは、上限を超えた理由がバリデーション
// エラーとして拒否され、投稿が視界に残り、何も記録されないことを検証します。
func TestUnpublishPostUsecase_Execute_InvalidReason(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Number:   1,
		Reason:   strings.Repeat("あ", validator.ModerationReasonMaxLength+1),
	})

	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasFieldError("reason") {
		t.Errorf("reasonのフィールドエラーが無い: %#v", ve.Fields)
	}
	if findPost(t, f.db, f.thread.ID, 1).UnpublishedAt != nil {
		t.Error("拒否後のUnpublishedAt = 非nil、期待値 = nil")
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d、期待値 = 0", len(logs))
	}
}

// TestUnpublishPostUsecase_Execute_ForbiddenBeforeReasonは、投稿を非公開にできない
// 操作者が、理由の検査より先に拒否されることを検証します。
func TestUnpublishPostUsecase_Execute_ForbiddenBeforeReason(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
		Actor:    usecase.UserActor(testutil.NewUserBuilder(t, f.db).Build()),
		ThreadID: f.thread.ID,
		Number:   1,
		Reason:   strings.Repeat("あ", validator.ModerationReasonMaxLength+1),
	})

	assertAppErrCode(t, err, model.AppErrCodeForbidden)
}

// TestUnpublishPostUsecase_Execute_Operatorは、サブコマンドを実行する運用者が投稿を
// 非公開にし、利用者idを持たない形で記録されることを検証します。運用者を記述する行は
// データベースに無いためです。
func TestUnpublishPostUsecase_Execute_Operator(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	if err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
		Actor:    usecase.OperatorActor(),
		ThreadID: f.thread.ID,
		Number:   1,
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 1 {
		t.Fatalf("操作履歴の件数 = %d、期待値 = 1", len(logs))
	}
	if logs[0].UserID != nil {
		t.Errorf("UserID = %v、期待値 = nil", logs[0].UserID)
	}
	if logs[0].Reason != "" {
		t.Errorf("Reason = %q、期待値 = 空文字列", logs[0].Reason)
	}
}

// TestUnpublishPostUsecase_Execute_KeepsPostLimitReachedは、持てる投稿をすべて
// 持っているスレッドが、その投稿の1つが非公開になった後も閉じたままであることを検証
// します。件数は表示されている本文の数ではなく発行したレス番号の数であるため、本文を視界
// から外してもスレッドに再び発行できる番号が渡るわけではありません。
func TestUnpublishPostUsecase_Execute_KeepsPostLimitReached(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	filled := growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit)
	if reasons := filled.LockReasons(); len(reasons) != 1 || reasons[0] != model.ThreadLockReasonPostLimitReached {
		t.Fatalf("非公開前のLockReasons() = %v、期待値 = [%s]", reasons, model.ThreadLockReasonPostLimitReached)
	}

	if err := f.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Number:   model.ThreadPostLimit,
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != model.ThreadPostLimit {
		t.Errorf("PostsCount = %d、期待値 = %d", thread.PostsCount, model.ThreadPostLimit)
	}
	if thread.LastPostID == nil || *thread.LastPostID != *filled.LastPostID {
		t.Errorf("LastPostID = %v、期待値 = %v", thread.LastPostID, filled.LastPostID)
	}
	if reasons := thread.LockReasons(); len(reasons) != 1 || reasons[0] != model.ThreadLockReasonPostLimitReached {
		t.Errorf("非公開後のLockReasons() = %v、期待値 = [%s]", reasons, model.ThreadLockReasonPostLimitReached)
	}
}
