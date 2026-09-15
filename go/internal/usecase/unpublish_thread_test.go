package usecase_test

import (
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestUnpublishThreadUsecase_Execute_Successは、管理者がスレッドをコミュニティの
// 視界から外せること、スレッドを並べていた掲示板が名指されること、スレッドが持っているものを
// すべて保つこと、そして履歴が書かれた理由とともにその操作を運ぶことを検証します。
func TestUnpublishThreadUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	output, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Reason:   "  掲示板の趣旨から外れているため  ",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}
	if output.BoardSlug != f.board.Slug {
		t.Errorf("BoardSlug = %q、期待値 = %q", output.BoardSlug, f.board.Slug)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.UnpublishedAt == nil {
		t.Fatal("非公開後のUnpublishedAt = nil、期待値 = 非nil")
	}
	if thread.Title != f.thread.Title {
		t.Errorf("Title = %q、期待値 = %q", thread.Title, f.thread.Title)
	}
	if thread.PostsCount != f.thread.PostsCount {
		t.Errorf("PostsCount = %d、期待値 = %d", thread.PostsCount, f.thread.PostsCount)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d、期待値 = 1", got)
	}
	if thread.LockedAt != nil {
		t.Errorf("LockedAt = %v、期待値 = nil", thread.LockedAt)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 1 {
		t.Fatalf("操作履歴の件数 = %d、期待値 = 1", len(logs))
	}
	log := logs[0]
	if log.Action != model.ModerationActionThreadUnpublish {
		t.Errorf("Action = %q、期待値 = %q", log.Action, model.ModerationActionThreadUnpublish)
	}
	if log.UserID == nil || *log.UserID != f.admin {
		t.Errorf("UserID = %v、期待値 = %s", log.UserID, f.admin)
	}
	if log.ThreadID == nil || *log.ThreadID != f.thread.ID {
		t.Errorf("ThreadID = %v、期待値 = %s", log.ThreadID, f.thread.ID)
	}
	if log.Reason != "掲示板の趣旨から外れているため" {
		t.Errorf("Reason = %q、期待値 = %q", log.Reason, "掲示板の趣旨から外れているため")
	}
	if log.PostID != nil || log.TargetUserID != nil {
		t.Errorf("PostID = %v、TargetUserID = %v、期待値 = どちらもnil", log.PostID, log.TargetUserID)
	}
}

// TestUnpublishThreadUsecase_Execute_ScopedActorは、この操作を許すのが
// thread_unpublication:writeであること、そして別のモデレーションのスコープだけを持つ
// ロールが拒否されることを検証します。
func TestUnpublishThreadUsecase_Execute_ScopedActor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roleName     model.RoleName
		scopes       []model.Scope
		wantAdmitted bool
	}{
		{
			name:         "thread_unpublication:writeを持つロールは許される",
			roleName:     "thread_unpublisher",
			scopes:       []model.Scope{model.ScopeThreadUnpublicationWrite},
			wantAdmitted: true,
		},
		{
			name:     "別のモデレーションのスコープだけでは拒否される",
			roleName: "thread_locker",
			scopes:   []model.Scope{model.ScopeThreadLockWrite},
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

			_, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
				Actor:    usecase.UserActor(actorID),
				ThreadID: f.thread.ID,
			})

			if tt.wantAdmitted {
				if err != nil {
					t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
				}
				if findThread(t, f.db, f.thread.ID).UnpublishedAt == nil {
					t.Error("非公開後のUnpublishedAt = nil、期待値 = 非nil")
				}
				return
			}

			assertAppErrCode(t, err, model.AppErrCodeForbidden)
			if findThread(t, f.db, f.thread.ID).UnpublishedAt != nil {
				t.Error("拒否後のUnpublishedAt = 非nil、期待値 = nil")
			}
			if logs := listModerationLogs(t, f.db); len(logs) != 0 {
				t.Errorf("拒否後の操作履歴の件数 = %d、期待値 = 0", len(logs))
			}
		})
	}
}

// TestUnpublishThreadUsecase_Execute_UnknownThreadは、コミュニティが持たない
// スレッドの非公開が、リソースの不在として答えられることを検証します。既に非公開である
// ことは成功ですが、そもそも存在しなかったことは成功ではありません。
func TestUnpublishThreadUsecase_Execute_UnknownThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	_, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID + 1000,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d、期待値 = 0", len(logs))
	}
}

// TestUnpublishThreadUsecase_Execute_AlreadyUnpublishedは、既に非公開のスレッドの
// 非公開が、履歴に2件目を足さず、視界から外された時刻も動かさずに成功すること、そして
// スレッドが立っていた掲示板を変わらず名指すことを検証します。既に成立している状態を求める
// 要求への答えは、同じ答えであるためです。
func TestUnpublishThreadUsecase_Execute_AlreadyUnpublished(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	input := usecase.UnpublishThreadInput{Actor: usecase.UserActor(f.admin), ThreadID: f.thread.ID}

	if _, err := f.unpublishThreadUC.Execute(ctx, input); err != nil {
		t.Fatalf("1度目のExecute()のエラー = %v、期待値 = nil", err)
	}
	unpublishedAt := findThread(t, f.db, f.thread.ID).UnpublishedAt

	second, err := f.unpublishThreadUC.Execute(ctx, input)
	if err != nil {
		t.Fatalf("2度目のExecute()のエラー = %v、期待値 = nil", err)
	}
	if second.BoardSlug != f.board.Slug {
		t.Errorf("2度目のBoardSlug = %q、期待値 = %q", second.BoardSlug, f.board.Slug)
	}

	if got := findThread(t, f.db, f.thread.ID).UnpublishedAt; got == nil || !got.Equal(*unpublishedAt) {
		t.Errorf("2度目の後のUnpublishedAt = %v、期待値 = %v", got, unpublishedAt)
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 1 {
		t.Errorf("操作履歴の件数 = %d、期待値 = 1", len(logs))
	}
}

// TestUnpublishThreadUsecase_Execute_InvalidReasonは、上限を超えた理由が
// バリデーションエラーとして拒否され、スレッドが視界に残り、何も記録されないことを検証
// します。
func TestUnpublishThreadUsecase_Execute_InvalidReason(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	_, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Reason:   strings.Repeat("あ", validator.ModerationReasonMaxLength+1),
	})

	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasFieldError("reason") {
		t.Errorf("reasonのフィールドエラーが無い: %#v", ve.Fields)
	}
	if findThread(t, f.db, f.thread.ID).UnpublishedAt != nil {
		t.Error("拒否後のUnpublishedAt = 非nil、期待値 = nil")
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d、期待値 = 0", len(logs))
	}
}

// TestUnpublishThreadUsecase_Execute_ForbiddenBeforeReasonは、スレッドを非公開に
// できない操作者が、理由の検査より先に拒否されることを検証します。
func TestUnpublishThreadUsecase_Execute_ForbiddenBeforeReason(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	_, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
		Actor:    usecase.UserActor(testutil.NewUserBuilder(t, f.db).Build()),
		ThreadID: f.thread.ID,
		Reason:   strings.Repeat("あ", validator.ModerationReasonMaxLength+1),
	})

	assertAppErrCode(t, err, model.AppErrCodeForbidden)
}

// TestUnpublishThreadUsecase_Execute_Operatorは、サブコマンドを実行する運用者が
// スレッドを非公開にし、利用者idを持たない形で記録されることを検証します。運用者を記述
// する行はデータベースに無いためです。
func TestUnpublishThreadUsecase_Execute_Operator(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	if _, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
		Actor:    usecase.OperatorActor(),
		ThreadID: f.thread.ID,
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

// TestUnpublishThreadUsecase_Execute_LockedThreadは、ロック中のスレッドが、
// ロックを乱されずに非公開になることを検証します。2つの印は別のことを述べており、
// スレッドを視界から外すことがスレッドを開き直すわけではありません。
func TestUnpublishThreadUsecase_Execute_LockedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("ロックに失敗: %v", err)
	}
	lockedAt := findThread(t, f.db, f.thread.ID).LockedAt

	if _, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.UnpublishedAt == nil {
		t.Error("非公開後のUnpublishedAt = nil、期待値 = 非nil")
	}
	if got := thread.LockedAt; got == nil || !got.Equal(*lockedAt) {
		t.Errorf("非公開後のLockedAt = %v、期待値 = %v", got, lockedAt)
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 2 {
		t.Errorf("操作履歴の件数 = %d、期待値 = 2", len(logs))
	}
}
