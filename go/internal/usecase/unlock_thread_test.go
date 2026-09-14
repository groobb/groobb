package usecase_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestUnlockThreadUsecase_Execute_Success verifies that an administrator lifts
// the lock they placed, that the thread then takes replies
// again, and that the history carries the operation with no reason of its own.
//
// [Ja] TestUnlockThreadUsecase_Execute_Successは、管理者が自身の掛けたロックを
// 外せること、スレッドが再び返信を受け付けること、そして履歴が自身の理由を持たない形で
// その操作を運ぶことを検証します。
func TestUnlockThreadUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Reason:   "様子を見るため",
	}); err != nil {
		t.Fatalf("前提のロックに失敗: %v", err)
	}

	if err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.LockedAt != nil {
		t.Errorf("解除後の LockedAt = %v, want nil", thread.LockedAt)
	}
	if reasons := thread.LockReasons(); len(reasons) != 0 {
		t.Errorf("解除後の LockReasons() = %v, want 空", reasons)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 2 {
		t.Fatalf("操作履歴の件数 = %d, want 2", len(logs))
	}
	log := logs[0]
	if log.Action != model.ModerationActionThreadUnlock {
		t.Errorf("Action = %q, want %q", log.Action, model.ModerationActionThreadUnlock)
	}
	if log.UserID == nil || *log.UserID != f.admin {
		t.Errorf("UserID = %v, want %s", log.UserID, f.admin)
	}
	if log.ThreadID == nil || *log.ThreadID != f.thread.ID {
		t.Errorf("ThreadID = %v, want %s", log.ThreadID, f.thread.ID)
	}
	if log.Reason != "" {
		t.Errorf("Reason = %q, want 空文字列", log.Reason)
	}
}

// TestUnlockThreadUsecase_Execute_KeepsThePostLimit verifies that lifting the
// administrators' lock leaves the cap in place: the thread is still locked, for
// the reason it arrived at by being written to rather than by a decision.
//
// [Ja] TestUnlockThreadUsecase_Execute_KeepsThePostLimitは、管理者のロックを外しても上限
// 到達が残ることを検証します。スレッドはなおロック中で、その理由は判断によってではなく
// 書き込まれることによって到達した状態です。
func TestUnlockThreadUsecase_Execute_KeepsThePostLimit(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit)
	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("前提のロックに失敗: %v", err)
	}

	if err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.LockedAt != nil {
		t.Errorf("解除後の LockedAt = %v, want nil", thread.LockedAt)
	}
	reasons := thread.LockReasons()
	if len(reasons) != 1 || reasons[0] != model.ThreadLockReasonPostLimitReached {
		t.Errorf("解除後の LockReasons() = %v, want [%s]", reasons, model.ThreadLockReasonPostLimitReached)
	}
}

// TestUnlockThreadUsecase_Execute_ScopedActor verifies that thread_lock:write is
// what admits lifting a lock as well, so the pair is one permission boundary,
// and that another moderation scope alone is refused.
//
// [Ja] TestUnlockThreadUsecase_Execute_ScopedActorは、解除を許すのもthread_lock:writeで
// あること (対の操作が1つの権限境界であること)、そして別のモデレーションのスコープだけでは
// 拒否されることを検証します。
func TestUnlockThreadUsecase_Execute_ScopedActor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roleName     model.RoleName
		scopes       []model.Scope
		wantAdmitted bool
	}{
		{
			name:         "thread_lock:write を持つロールは許される",
			roleName:     "thread_locker",
			scopes:       []model.Scope{model.ScopeThreadLockWrite},
			wantAdmitted: true,
		},
		{
			name:     "別のモデレーションのスコープだけでは拒否される",
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
			if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
				Actor:    usecase.UserActor(f.admin),
				ThreadID: f.thread.ID,
			}); err != nil {
				t.Fatalf("前提のロックに失敗: %v", err)
			}
			actorID := seedScopedActor(t, f.db, tt.roleName, tt.scopes)

			err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
				Actor:    usecase.UserActor(actorID),
				ThreadID: f.thread.ID,
			})

			if tt.wantAdmitted {
				if err != nil {
					t.Fatalf("Execute() error = %v, want nil", err)
				}
				if findThread(t, f.db, f.thread.ID).LockedAt != nil {
					t.Error("解除後の LockedAt = 非nil, want nil")
				}
				return
			}

			assertAppErrCode(t, err, model.AppErrCodeForbidden)
			if findThread(t, f.db, f.thread.ID).LockedAt == nil {
				t.Error("拒否後の LockedAt = nil, want 非nil")
			}
			if logs := listModerationLogs(t, f.db); len(logs) != 1 {
				t.Errorf("拒否後の操作履歴の件数 = %d, want 1", len(logs))
			}
		})
	}
}

// TestUnlockThreadUsecase_Execute_UnknownThread verifies that lifting a lock
// from a thread the community does not have is answered as a missing resource.
//
// [Ja] TestUnlockThreadUsecase_Execute_UnknownThreadは、コミュニティが持たないスレッドの
// ロックの解除が、リソースの不在として答えられることを検証します。
func TestUnlockThreadUsecase_Execute_UnknownThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID + 1000,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d, want 0", len(logs))
	}
}

// TestUnlockThreadUsecase_Execute_UnpublishedThread verifies that an unpublished
// thread is refused with its own code: what a lift would restore is a thread the
// community is not shown either way.
//
// [Ja] TestUnlockThreadUsecase_Execute_UnpublishedThreadは、非公開のスレッドが専用の
// コードで拒否されることを検証します。解除が戻すのは、どのみちコミュニティに示されない
// スレッドであるためです。
func TestUnlockThreadUsecase_Execute_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("前提のロックに失敗: %v", err)
	}
	unpublishThread(t, f.db, f.thread.ID)

	err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceUnpublished)
	if findThread(t, f.db, f.thread.ID).LockedAt == nil {
		t.Error("拒否後の LockedAt = nil, want 非nil")
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 1 {
		t.Errorf("操作履歴の件数 = %d, want 1", len(logs))
	}
}

// TestUnlockThreadUsecase_Execute_NotLocked verifies that lifting a lock nobody
// placed succeeds without recording anything, whether the thread is open or
// locked by its post cap alone. What the request asked for is that the
// administrators' lock not hold, and it does not.
//
// [Ja] TestUnlockThreadUsecase_Execute_NotLockedは、誰も掛けていないロックの解除が、何も
// 記録せずに成功することを検証します。スレッドが開いている場合も、投稿数の上限だけで
// ロックされている場合も同じです。要求が求めたのは管理者のロックが成立していないことであり、
// 実際に成立していないためです。
func TestUnlockThreadUsecase_Execute_NotLocked(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fillToCap bool
	}{
		{name: "開いているスレッド"},
		{name: "上限到達だけでロックされているスレッド", fillToCap: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, ctx := newThreadModerationUsecases(t)
			if tt.fillToCap {
				growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit)
			}

			if err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
				Actor:    usecase.UserActor(f.admin),
				ThreadID: f.thread.ID,
			}); err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}

			if got := findThread(t, f.db, f.thread.ID).LockedAt; got != nil {
				t.Errorf("LockedAt = %v, want nil", got)
			}
			if logs := listModerationLogs(t, f.db); len(logs) != 0 {
				t.Errorf("操作履歴の件数 = %d, want 0", len(logs))
			}
		})
	}
}

// TestUnlockThreadUsecase_Execute_Operator verifies that an operator running a
// subcommand lifts the lock and is recorded with no user id.
//
// [Ja] TestUnlockThreadUsecase_Execute_Operatorは、サブコマンドを実行する運用者がロックを
// 外し、利用者idを持たない形で記録されることを検証します。
func TestUnlockThreadUsecase_Execute_Operator(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.OperatorActor(),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("前提のロックに失敗: %v", err)
	}

	if err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
		Actor:    usecase.OperatorActor(),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 2 {
		t.Fatalf("操作履歴の件数 = %d, want 2", len(logs))
	}
	if logs[0].UserID != nil {
		t.Errorf("UserID = %v, want nil", logs[0].UserID)
	}
	if logs[0].Action != model.ModerationActionThreadUnlock {
		t.Errorf("Action = %q, want %q", logs[0].Action, model.ModerationActionThreadUnlock)
	}
}

// TestUnlockThreadUsecase_Execute_ActorWithoutRole verifies that an account
// holding no role is refused, which is what resolveCommunityPolicy answers for
// anyone the roles lookup finds nothing for.
//
// [Ja] TestUnlockThreadUsecase_Execute_ActorWithoutRoleは、ロールを1つも持たない
// アカウントが拒否されることを検証します。ロールのルックアップが何も見つけない相手に対して
// resolveCommunityPolicyが答えるのがこれです。
func TestUnlockThreadUsecase_Execute_ActorWithoutRole(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	err := f.unlockUC.Execute(ctx, usecase.UnlockThreadInput{
		Actor:    usecase.UserActor(testutil.NewUserBuilder(t, f.db).Build()),
		ThreadID: f.thread.ID,
	})

	assertAppErrCode(t, err, model.AppErrCodeForbidden)
}
