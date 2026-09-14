package usecase_test

import (
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestUnpublishThreadUsecase_Execute_Success verifies that an administrator
// takes a thread out of the community's view, that it names the board that
// listed the thread, that the thread keeps everything it holds, and that the
// history carries the operation with the reason that was written.
//
// [Ja] TestUnpublishThreadUsecase_Execute_Successは、管理者がスレッドをコミュニティの
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
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if output.BoardSlug != f.board.Slug {
		t.Errorf("BoardSlug = %q, want %q", output.BoardSlug, f.board.Slug)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.UnpublishedAt == nil {
		t.Fatal("非公開後の UnpublishedAt = nil, want 非nil")
	}
	if thread.Title != f.thread.Title {
		t.Errorf("Title = %q, want %q", thread.Title, f.thread.Title)
	}
	if thread.PostsCount != f.thread.PostsCount {
		t.Errorf("PostsCount = %d, want %d", thread.PostsCount, f.thread.PostsCount)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want 1", got)
	}
	if thread.LockedAt != nil {
		t.Errorf("LockedAt = %v, want nil", thread.LockedAt)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 1 {
		t.Fatalf("操作履歴の件数 = %d, want 1", len(logs))
	}
	log := logs[0]
	if log.Action != model.ModerationActionThreadUnpublish {
		t.Errorf("Action = %q, want %q", log.Action, model.ModerationActionThreadUnpublish)
	}
	if log.UserID == nil || *log.UserID != f.admin {
		t.Errorf("UserID = %v, want %s", log.UserID, f.admin)
	}
	if log.ThreadID == nil || *log.ThreadID != f.thread.ID {
		t.Errorf("ThreadID = %v, want %s", log.ThreadID, f.thread.ID)
	}
	if log.Reason != "掲示板の趣旨から外れているため" {
		t.Errorf("Reason = %q, want %q", log.Reason, "掲示板の趣旨から外れているため")
	}
	if log.PostID != nil || log.TargetUserID != nil {
		t.Errorf("PostID = %v, TargetUserID = %v, want どちらも nil", log.PostID, log.TargetUserID)
	}
}

// TestUnpublishThreadUsecase_Execute_ScopedActor verifies that
// thread_unpublication:write is what admits the operation, and that a role
// carrying another moderation scope alone is refused.
//
// [Ja] TestUnpublishThreadUsecase_Execute_ScopedActorは、この操作を許すのが
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
			name:         "thread_unpublication:write を持つロールは許される",
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
					t.Fatalf("Execute() error = %v, want nil", err)
				}
				if findThread(t, f.db, f.thread.ID).UnpublishedAt == nil {
					t.Error("非公開後の UnpublishedAt = nil, want 非nil")
				}
				return
			}

			assertAppErrCode(t, err, model.AppErrCodeForbidden)
			if findThread(t, f.db, f.thread.ID).UnpublishedAt != nil {
				t.Error("拒否後の UnpublishedAt = 非nil, want nil")
			}
			if logs := listModerationLogs(t, f.db); len(logs) != 0 {
				t.Errorf("拒否後の操作履歴の件数 = %d, want 0", len(logs))
			}
		})
	}
}

// TestUnpublishThreadUsecase_Execute_UnknownThread verifies that unpublishing a
// thread the community does not have is answered as a missing resource. Being
// already unpublished is success, but never having existed is not.
//
// [Ja] TestUnpublishThreadUsecase_Execute_UnknownThreadは、コミュニティが持たない
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
		t.Errorf("操作履歴の件数 = %d, want 0", len(logs))
	}
}

// TestUnpublishThreadUsecase_Execute_AlreadyUnpublished verifies that
// unpublishing a thread that is already unpublished succeeds without adding a
// second entry to the history and without moving the time it was taken out of
// view, and that it still names the board the thread was posted in: the answer
// to a request asking for a state that already holds is the same answer.
//
// [Ja] TestUnpublishThreadUsecase_Execute_AlreadyUnpublishedは、既に非公開のスレッドの
// 非公開が、履歴に2件目を足さず、視界から外された時刻も動かさずに成功すること、そして
// スレッドが立っていた掲示板を変わらず名指すことを検証します。既に成立している状態を求める
// 要求への答えは、同じ答えであるためです。
func TestUnpublishThreadUsecase_Execute_AlreadyUnpublished(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	input := usecase.UnpublishThreadInput{Actor: usecase.UserActor(f.admin), ThreadID: f.thread.ID}

	if _, err := f.unpublishThreadUC.Execute(ctx, input); err != nil {
		t.Fatalf("1度目の Execute() error = %v, want nil", err)
	}
	unpublishedAt := findThread(t, f.db, f.thread.ID).UnpublishedAt

	second, err := f.unpublishThreadUC.Execute(ctx, input)
	if err != nil {
		t.Fatalf("2度目の Execute() error = %v, want nil", err)
	}
	if second.BoardSlug != f.board.Slug {
		t.Errorf("2度目の BoardSlug = %q, want %q", second.BoardSlug, f.board.Slug)
	}

	if got := findThread(t, f.db, f.thread.ID).UnpublishedAt; got == nil || !got.Equal(*unpublishedAt) {
		t.Errorf("2度目の後の UnpublishedAt = %v, want %v", got, unpublishedAt)
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 1 {
		t.Errorf("操作履歴の件数 = %d, want 1", len(logs))
	}
}

// TestUnpublishThreadUsecase_Execute_InvalidReason verifies that a reason over
// the limit is refused as a validation error, with the thread left in view and
// nothing recorded.
//
// [Ja] TestUnpublishThreadUsecase_Execute_InvalidReasonは、上限を超えた理由が
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
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasFieldError("reason") {
		t.Errorf("reason のフィールドエラーが無い: %#v", ve.Fields)
	}
	if findThread(t, f.db, f.thread.ID).UnpublishedAt != nil {
		t.Error("拒否後の UnpublishedAt = 非nil, want nil")
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d, want 0", len(logs))
	}
}

// TestUnpublishThreadUsecase_Execute_ForbiddenBeforeReason verifies that an
// actor who may not unpublish threads is refused before the reason is examined.
//
// [Ja] TestUnpublishThreadUsecase_Execute_ForbiddenBeforeReasonは、スレッドを非公開に
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

// TestUnpublishThreadUsecase_Execute_Operator verifies that an operator running
// a subcommand unpublishes the thread and is recorded with no user id: no row in
// the database describes them.
//
// [Ja] TestUnpublishThreadUsecase_Execute_Operatorは、サブコマンドを実行する運用者が
// スレッドを非公開にし、利用者idを持たない形で記録されることを検証します。運用者を記述
// する行はデータベースに無いためです。
func TestUnpublishThreadUsecase_Execute_Operator(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	if _, err := f.unpublishThreadUC.Execute(ctx, usecase.UnpublishThreadInput{
		Actor:    usecase.OperatorActor(),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 1 {
		t.Fatalf("操作履歴の件数 = %d, want 1", len(logs))
	}
	if logs[0].UserID != nil {
		t.Errorf("UserID = %v, want nil", logs[0].UserID)
	}
	if logs[0].Reason != "" {
		t.Errorf("Reason = %q, want 空文字列", logs[0].Reason)
	}
}

// TestUnpublishThreadUsecase_Execute_LockedThread verifies that a locked thread
// is unpublished without the lock being disturbed: the two marks say different
// things, and taking the thread out of view does not reopen it.
//
// [Ja] TestUnpublishThreadUsecase_Execute_LockedThreadは、ロック中のスレッドが、
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
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.UnpublishedAt == nil {
		t.Error("非公開後の UnpublishedAt = nil, want 非nil")
	}
	if got := thread.LockedAt; got == nil || !got.Equal(*lockedAt) {
		t.Errorf("非公開後の LockedAt = %v, want %v", got, lockedAt)
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 2 {
		t.Errorf("操作履歴の件数 = %d, want 2", len(logs))
	}
}
