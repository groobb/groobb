package usecase_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// threadModerationFixture is the UseCases under test together with the board and
// thread they act on, the administrator acting, and the database the assertions
// read the committed rows back from.
//
// The phase's four operations share it because each acts on the same thing: one
// thread the community shows. A test about one of them has to set up what
// another leaves behind, as a lift is given a lock, and as an operation refused
// on an unpublished thread is given an unpublication.
//
// [Ja] threadModerationFixtureは、テスト対象のUseCaseと、それらが操作する掲示板と
// スレッド、操作する管理者、そして検証がコミットされた行を読み戻すデータベースです。
//
// 本フェーズの4つの操作がこれを共有するのは、いずれも同じもの、すなわちコミュニティが
// 示している1つのスレッドに対して行われるためです。ある操作のテストは別の操作が残すものを
// 用意することになります。解除に与えられるのはロックであり、非公開のスレッドで拒否される
// 操作に与えられるのは非公開です。
type threadModerationFixture struct {
	db                *database.DB
	lockUC            *usecase.LockThreadUsecase
	unlockUC          *usecase.UnlockThreadUsecase
	unpublishThreadUC *usecase.UnpublishThreadUsecase
	unpublishPostUC   *usecase.UnpublishPostUsecase
	getModerationUC   *usecase.GetThreadModerationUsecase
	board             *model.Board
	thread            *model.Thread
	starter           model.UserID
	admin             model.UserID
}

// newThreadModerationUsecases builds the UseCases over a database holding one
// board, one thread with its first post, and an administrator to act as.
//
// [Ja] newThreadModerationUsecasesは、掲示板が1つ、最初の投稿を持つスレッドが1つ、そして
// 操作する管理者がいるデータベース上に、UseCaseを構築します。
func newThreadModerationUsecases(t *testing.T) (threadModerationFixture, context.Context) {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	return newThreadModerationUsecasesOn(t, testutil.SetupDB(t)), ctx
}

// newThreadModerationUsecasesOn seeds the board, thread and administrator into
// db and builds the UseCases over it. It takes the database rather than opening
// one so that a test can hand it a pool it opened itself.
//
// [Ja] newThreadModerationUsecasesOnは掲示板・スレッド・管理者をdbに投入し、その上に
// UseCaseを構築します。データベースを開かずに受け取るのは、テストが自分で開いたプールを
// 渡せるようにするためです。
func newThreadModerationUsecasesOn(t *testing.T, db *database.DB) threadModerationFixture {
	t.Helper()

	ctx := context.Background()
	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	category, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("テスト用カテゴリーの作成に失敗: %v", err)
	}
	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &category.ID, Slug: "jazz", Name: "ジャズ"})
	if err != nil {
		t.Fatalf("テスト用掲示板の作成に失敗: %v", err)
	}

	starter := testutil.NewUserBuilder(t, db).Build()
	thread, _ := seedThread(t, db, board.ID, starter, "枯葉の名演")

	return threadModerationFixture{
		db:                db,
		lockUC:            newLockThreadUsecase(db),
		unlockUC:          newUnlockThreadUsecase(db),
		unpublishThreadUC: newUnpublishThreadUsecase(db),
		unpublishPostUC:   newUnpublishPostUsecase(db),
		getModerationUC:   newGetThreadModerationUsecase(db),
		board:             board,
		thread:            thread,
		starter:           starter,
		admin:             seedAdmin(t, db),
	}
}

// newLockThreadUsecase wires a LockThreadUsecase over db's pools. The UseCase
// opens its own transaction, so a test asserts against the rows it commits.
//
// [Ja] newLockThreadUsecaseはdbのプール上にLockThreadUsecaseを組み立てます。UseCaseは
// 自前のトランザクションを開くため、テストはそれがコミットした行を検証します。
func newLockThreadUsecase(db *database.DB) *usecase.LockThreadUsecase {
	return usecase.NewLockThreadUsecase(
		db.Writer,
		validator.NewModerationLogCreateValidator(),
		repository.NewRoleRepository(db),
		repository.NewThreadRepository(db),
		repository.NewModerationLogRepository(db),
	)
}

// newUnlockThreadUsecase wires an UnlockThreadUsecase over db's pools.
//
// [Ja] newUnlockThreadUsecaseはdbのプール上にUnlockThreadUsecaseを組み立てます。
func newUnlockThreadUsecase(db *database.DB) *usecase.UnlockThreadUsecase {
	return usecase.NewUnlockThreadUsecase(
		db.Writer,
		repository.NewRoleRepository(db),
		repository.NewThreadRepository(db),
		repository.NewModerationLogRepository(db),
	)
}

// newUnpublishThreadUsecase wires an UnpublishThreadUsecase over db's pools.
//
// [Ja] newUnpublishThreadUsecaseはdbのプール上にUnpublishThreadUsecaseを組み立てます。
func newUnpublishThreadUsecase(db *database.DB) *usecase.UnpublishThreadUsecase {
	return usecase.NewUnpublishThreadUsecase(
		db.Writer,
		validator.NewModerationLogCreateValidator(),
		repository.NewRoleRepository(db),
		repository.NewThreadRepository(db),
		repository.NewBoardRepository(db),
		repository.NewModerationLogRepository(db),
	)
}

// newUnpublishPostUsecase wires an UnpublishPostUsecase over db's pools.
//
// [Ja] newUnpublishPostUsecaseはdbのプール上にUnpublishPostUsecaseを組み立てます。
func newUnpublishPostUsecase(db *database.DB) *usecase.UnpublishPostUsecase {
	return usecase.NewUnpublishPostUsecase(
		db.Writer,
		validator.NewModerationLogCreateValidator(),
		repository.NewRoleRepository(db),
		repository.NewThreadRepository(db),
		repository.NewPostRepository(db),
		repository.NewModerationLogRepository(db),
	)
}

// newGetThreadModerationUsecase wires a GetThreadModerationUsecase over db's
// pools. It reads, so a test gives it the state the other three leave behind.
//
// [Ja] newGetThreadModerationUsecaseはdbのプール上にGetThreadModerationUsecaseを
// 組み立てます。これは読み取るものであるため、テストは他の3つが残す状態をこれに与えます。
func newGetThreadModerationUsecase(db *database.DB) *usecase.GetThreadModerationUsecase {
	return usecase.NewGetThreadModerationUsecase(
		repository.NewRoleRepository(db),
		repository.NewThreadRepository(db),
		repository.NewPostRepository(db),
		repository.NewUserRepository(db),
	)
}

// listModerationLogs returns the whole history, the most recent operation
// first. A test reads it to state what an operation recorded, and to state that
// an operation that changed nothing recorded nothing.
//
// [Ja] listModerationLogsは履歴の全体を、新しい操作から順に返します。テストは、ある操作が
// 何を記録したかを述べるため、そして何も変えなかった操作が何も記録しなかったことを述べる
// ために、これを読みます。
func listModerationLogs(t *testing.T, db *database.DB) []*model.ModerationLog {
	t.Helper()

	logs, err := repository.NewModerationLogRepository(db).ListPage(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("操作履歴の取得に失敗: %v", err)
	}
	return logs
}

// seedScopedActor creates a user holding a role that grants exactly the given
// scopes, for a test stating what one scope admits and what it does not.
//
// [Ja] seedScopedActorは、与えたスコープだけを持つロールを保持する利用者を作ります。
// 1つのスコープが何を許し何を許さないかを述べるテストのためのものです。
func seedScopedActor(t *testing.T, db *database.DB, name model.RoleName, scopes []model.Scope) model.UserID {
	t.Helper()

	testutil.NewRoleBuilder(t, db).WithName(name).WithScopes(scopes).Build()
	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).WithRoleName(name).Build()
	return userID
}

// unpublishThread marks the fixture's thread unpublished, which is the state the
// operations acting on what the community shows are refused on. Unpublishing the
// thread is the one that is not, being the operation that puts it in this state.
//
// [Ja] unpublishThreadはフィクスチャのスレッドに非公開の印を付けます。コミュニティの
// 示しているものに対して行われる操作が拒否される状態です。スレッドの非公開だけは拒否され
// ません。スレッドをこの状態に置く操作そのものであるためです。
func unpublishThread(t *testing.T, db *database.DB, id model.ThreadID) {
	t.Helper()

	if err := repository.NewThreadRepository(db).Unpublish(context.Background(), id); err != nil {
		t.Fatalf("テスト用スレッドの非公開に失敗: %v", err)
	}
}

// TestLockThreadUsecase_Execute_Success verifies that an administrator closes a
// thread to new replies, that the thread then says so, and that the history
// carries the operation with the reason that was written.
//
// [Ja] TestLockThreadUsecase_Execute_Successは、管理者がスレッドを新しい返信に対して
// 締め切れること、スレッドがその旨を述べること、そして履歴が書かれた理由とともにその操作を
// 運ぶことを検証します。
func TestLockThreadUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Reason:   "  規約に反する書き込みが続いたため  ",
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.LockedAt == nil {
		t.Fatal("ロック後の LockedAt = nil, want 非nil")
	}
	if reasons := thread.LockReasons(); len(reasons) != 1 || reasons[0] != model.ThreadLockReasonLockedByModerator {
		t.Errorf("ロック後の LockReasons() = %v, want [%s]", reasons, model.ThreadLockReasonLockedByModerator)
	}

	logs := listModerationLogs(t, f.db)
	if len(logs) != 1 {
		t.Fatalf("操作履歴の件数 = %d, want 1", len(logs))
	}
	log := logs[0]
	if log.Action != model.ModerationActionThreadLock {
		t.Errorf("Action = %q, want %q", log.Action, model.ModerationActionThreadLock)
	}
	if log.UserID == nil || *log.UserID != f.admin {
		t.Errorf("UserID = %v, want %s", log.UserID, f.admin)
	}
	if log.ThreadID == nil || *log.ThreadID != f.thread.ID {
		t.Errorf("ThreadID = %v, want %s", log.ThreadID, f.thread.ID)
	}
	if log.Reason != "規約に反する書き込みが続いたため" {
		t.Errorf("Reason = %q, want %q", log.Reason, "規約に反する書き込みが続いたため")
	}
	if log.PostID != nil || log.TargetUserID != nil {
		t.Errorf("PostID = %v, TargetUserID = %v, want どちらも nil", log.PostID, log.TargetUserID)
	}
}

// TestLockThreadUsecase_Execute_ScopedActor verifies that thread_lock:write is
// what admits the lock, and that a role carrying another moderation scope alone
// is refused.
//
// [Ja] TestLockThreadUsecase_Execute_ScopedActorは、ロックを許すのがthread_lock:writeで
// あること、そして別のモデレーションのスコープだけを持つロールが拒否されることを検証します。
func TestLockThreadUsecase_Execute_ScopedActor(t *testing.T) {
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
			actorID := seedScopedActor(t, f.db, tt.roleName, tt.scopes)

			err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
				Actor:    usecase.UserActor(actorID),
				ThreadID: f.thread.ID,
			})

			if tt.wantAdmitted {
				if err != nil {
					t.Fatalf("Execute() error = %v, want nil", err)
				}
				if findThread(t, f.db, f.thread.ID).LockedAt == nil {
					t.Error("ロック後の LockedAt = nil, want 非nil")
				}
				return
			}

			assertAppErrCode(t, err, model.AppErrCodeForbidden)
			if findThread(t, f.db, f.thread.ID).LockedAt != nil {
				t.Error("拒否後の LockedAt = 非nil, want nil")
			}
			if logs := listModerationLogs(t, f.db); len(logs) != 0 {
				t.Errorf("拒否後の操作履歴の件数 = %d, want 0", len(logs))
			}
		})
	}
}

// TestLockThreadUsecase_Execute_UnknownThread verifies that locking a thread the
// community does not have is answered as a missing resource.
//
// [Ja] TestLockThreadUsecase_Execute_UnknownThreadは、コミュニティが持たないスレッドの
// ロックが、リソースの不在として答えられることを検証します。
func TestLockThreadUsecase_Execute_UnknownThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID + 1000,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d, want 0", len(logs))
	}
}

// TestLockThreadUsecase_Execute_UnpublishedThread verifies that an unpublished
// thread is refused with its own code rather than as a missing one: it is an
// address the community held and no longer shows, and it shows nothing to lock.
//
// [Ja] TestLockThreadUsecase_Execute_UnpublishedThreadは、非公開のスレッドが不在としてでは
// なく専用のコードで拒否されることを検証します。コミュニティが持っていて今は示さない
// アドレスであり、ロックする対象を何も示していないためです。
func TestLockThreadUsecase_Execute_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	unpublishThread(t, f.db, f.thread.ID)

	err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceUnpublished)
	if findThread(t, f.db, f.thread.ID).LockedAt != nil {
		t.Error("拒否後の LockedAt = 非nil, want nil")
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d, want 0", len(logs))
	}
}

// TestLockThreadUsecase_Execute_AlreadyLocked verifies that locking a thread an
// administrator already locked succeeds without adding a second entry to the
// history.
//
// [Ja] TestLockThreadUsecase_Execute_AlreadyLockedは、管理者が既にロックしたスレッドの
// ロックが、履歴に2件目を足さずに成功することを検証します。
func TestLockThreadUsecase_Execute_AlreadyLocked(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	input := usecase.LockThreadInput{Actor: usecase.UserActor(f.admin), ThreadID: f.thread.ID}

	if err := f.lockUC.Execute(ctx, input); err != nil {
		t.Fatalf("1度目の Execute() error = %v, want nil", err)
	}
	lockedAt := findThread(t, f.db, f.thread.ID).LockedAt

	if err := f.lockUC.Execute(ctx, input); err != nil {
		t.Fatalf("2度目の Execute() error = %v, want nil", err)
	}

	if got := findThread(t, f.db, f.thread.ID).LockedAt; got == nil || !got.Equal(*lockedAt) {
		t.Errorf("2度目の後の LockedAt = %v, want %v", got, lockedAt)
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 1 {
		t.Errorf("操作履歴の件数 = %d, want 1", len(logs))
	}
}

// TestLockThreadUsecase_Execute_InvalidReason verifies that a reason over the
// limit is refused as a validation error, with the thread left open and nothing
// recorded.
//
// [Ja] TestLockThreadUsecase_Execute_InvalidReasonは、上限を超えた理由がバリデーション
// エラーとして拒否され、スレッドが開いたまま、何も記録されないことを検証します。
func TestLockThreadUsecase_Execute_InvalidReason(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
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
	if findThread(t, f.db, f.thread.ID).LockedAt != nil {
		t.Error("拒否後の LockedAt = 非nil, want nil")
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 0 {
		t.Errorf("操作履歴の件数 = %d, want 0", len(logs))
	}
}

// TestLockThreadUsecase_Execute_ForbiddenBeforeReason verifies that an actor who
// may not lock threads is refused before the reason is examined, so a refusal
// does not turn into a request to shorten a note that would be refused anyway.
//
// [Ja] TestLockThreadUsecase_Execute_ForbiddenBeforeReasonは、スレッドをロックできない
// 操作者が理由の検査より先に拒否されることを検証します。拒否が、どのみち拒否される注記を
// 短くするようにという要求に変わらないためです。
func TestLockThreadUsecase_Execute_ForbiddenBeforeReason(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(testutil.NewUserBuilder(t, f.db).Build()),
		ThreadID: f.thread.ID,
		Reason:   strings.Repeat("あ", validator.ModerationReasonMaxLength+1),
	})

	assertAppErrCode(t, err, model.AppErrCodeForbidden)
}

// TestLockThreadUsecase_Execute_Operator verifies that an operator running a
// subcommand locks the thread and is recorded with no user id: no row in the
// database describes them.
//
// [Ja] TestLockThreadUsecase_Execute_Operatorは、サブコマンドを実行する運用者がスレッドを
// ロックし、利用者idを持たない形で記録されることを検証します。運用者を記述する行は
// データベースに無いためです。
func TestLockThreadUsecase_Execute_Operator(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
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

// TestLockThreadUsecase_Execute_ConcurrentWithReply verifies that a reply from
// another pool waits for an uncommitted lock and is refused after it commits.
// A reply committed before the lock could succeed; this test fixes the opposite
// order to exercise the refusal rather than accepting either outcome.
//
// [Ja] TestLockThreadUsecase_Execute_ConcurrentWithReplyは、別プールからの返信が未コミットの
// ロックを待ち、そのコミット後に拒否されることを検証します。返信が先にコミットされれば
// 成功し得ますが、このテストは逆の順序に固定し、拒否されることを確かめます。
func TestLockThreadUsecase_Execute_ConcurrentWithReply(t *testing.T) {
	t.Parallel()

	path := testutil.SetupDBPath(t)
	f := newThreadModerationUsecasesOn(t, openDB(t, path))
	replyDB := openDB(t, path)
	replyUC := newCreatePostUsecaseOver(replyDB)
	replier := testutil.NewUserBuilder(t, f.db).Build()
	ctx, cancel := context.WithTimeout(i18n.SetLocale(t.Context(), model.LocaleJa), 10*time.Second)
	defer cancel()

	lockReady := make(chan struct{})
	allowCommit := make(chan struct{})
	// The hook stays on the connection until the cleanup takes it off, so it
	// runs again for any commit that follows. Both signals are closed through a
	// sync.OnceFunc, so what keeps this test from panicking on a second close is
	// the signal itself rather than the order its assertions happen to be in.
	//
	// [Ja] フックはクリーンアップが外すまで接続に残るため、後続のコミットでも再び走る。
	// 2つの通知はどちらもsync.OnceFunc経由で閉じ、二重クローズでパニックしないことの根拠を、
	// 後続のアサーションの並びではなく通知そのものに持たせる。
	notifyLockReady := sync.OnceFunc(func() { close(lockReady) })
	resumeCommit := sync.OnceFunc(func() { close(allowCommit) })
	defer resumeCommit()

	// The hook belongs only to this test's writer connection. It pauses after
	// the thread and history are written, while SQLite still holds the write
	// lock, without adding a synchronization hook to the application itself.
	//
	// [Ja] フックはこのテストの書き込み接続にだけ設定する。スレッドと履歴を書き込んだ後、
	// SQLiteが書き込みロックを保持している間に停止させ、本番コードへの同期点の追加を避ける。
	conn, err := f.db.Writer.Conn(ctx)
	if err != nil {
		t.Fatalf("書き込み接続の取得に失敗: %v", err)
	}
	err = conn.Raw(func(raw any) error {
		hooks, ok := raw.(sqlite.HookRegisterer)
		if !ok {
			return fmt.Errorf("接続がSQLiteのフックに対応していない: %T", raw)
		}
		hooks.RegisterCommitHook(func() int32 {
			notifyLockReady()
			select {
			case <-allowCommit:
				return 0
			case <-ctx.Done():
				return 1
			}
		})
		return nil
	})
	closeErr := conn.Close()
	if err != nil {
		t.Fatalf("コミットフックの設定に失敗: %v", err)
	}
	if closeErr != nil {
		t.Fatalf("書き込み接続の返却に失敗: %v", closeErr)
	}

	t.Cleanup(func() {
		conn, err := f.db.Writer.Conn(context.Background())
		if err != nil {
			t.Errorf("フック解除用の接続取得に失敗: %v", err)
			return
		}
		if err := conn.Raw(func(raw any) error {
			raw.(sqlite.HookRegisterer).RegisterCommitHook(nil)
			return nil
		}); err != nil {
			t.Errorf("コミットフックの解除に失敗: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Errorf("フック解除用の接続返却に失敗: %v", err)
		}
	})

	lockResult := make(chan error, 1)
	go func() {
		lockResult <- f.lockUC.Execute(ctx, usecase.LockThreadInput{
			Actor:    usecase.UserActor(f.admin),
			ThreadID: f.thread.ID,
		})
	}()
	select {
	case <-lockReady:
	case err := <-lockResult:
		t.Fatalf("コミットの同期点へ到達せずロックが終了: %v", err)
	case <-ctx.Done():
		t.Fatalf("ロックの同期点を待機中にタイムアウト: %v", ctx.Err())
	}

	replyResult := make(chan error, 1)
	go func() {
		_, err := replyUC.Execute(ctx, usecase.CreatePostInput{
			ThreadID: f.thread.ID,
			UserID:   replier,
			Body:     "ロックのコミットを待つ返信",
		})
		replyResult <- err
	}()

	// InUse confirms the reply has taken its independent writer connection
	// before the lock is released. A goroutine-start signal alone would allow
	// the lock to commit before the reply even reaches the database.
	//
	// [Ja] InUseにより、ロックを解放する前に返信が独立した書き込み接続を取得したことを
	// 確認する。goroutineの起動通知だけでは、返信がDBへ到達する前にコミットされ得る。
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for replyDB.Writer.Stats().InUse == 0 {
		select {
		case err := <-replyResult:
			t.Fatalf("ロックのコミット前に返信が終了: %v", err)
		case <-ctx.Done():
			t.Fatalf("返信の書き込み接続を待機中にタイムアウト: %v", ctx.Err())
		case <-ticker.C:
		}
	}
	resumeCommit()

	if err := <-lockResult; err != nil {
		t.Fatalf("ロックの error = %v, want nil", err)
	}
	assertAppErrCode(t, <-replyResult, model.AppErrCodeThreadLocked)
	thread := findThread(t, f.db, f.thread.ID)
	if thread.LockedAt == nil {
		t.Error("ロック後の LockedAt = nil, want 非nil")
	}
	if thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d, want 1", thread.PostsCount)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want 1", got)
	}
	if logs := listModerationLogs(t, f.db); len(logs) != 1 || logs[0].Action != model.ModerationActionThreadLock {
		t.Errorf("操作履歴 = %v, want ロック1件", logs)
	}
}

// TestLockThreadUsecase_Execute_ReplyAfterCommit verifies that a reply sent
// through another pool after locking has completed is refused, even when its
// author is the administrator who locked the thread.
//
// [Ja] TestLockThreadUsecase_Execute_ReplyAfterCommitは、ロック完了後に別プールから送った
// 返信が、ロックした管理者自身の投稿も含めて拒否されることを検証します。
func TestLockThreadUsecase_Execute_ReplyAfterCommit(t *testing.T) {
	t.Parallel()

	path := testutil.SetupDBPath(t)
	f := newThreadModerationUsecasesOn(t, openDB(t, path))
	replyUC := newCreatePostUsecaseOver(openDB(t, path))
	ctx := i18n.SetLocale(t.Context(), model.LocaleJa)
	if err := f.lockUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	}); err != nil {
		t.Fatalf("ロックに失敗: %v", err)
	}

	for _, userID := range []model.UserID{f.starter, f.admin} {
		_, err := replyUC.Execute(ctx, usecase.CreatePostInput{
			ThreadID: f.thread.ID,
			UserID:   userID,
			Body:     "ロック完了後の返信",
		})
		assertAppErrCode(t, err, model.AppErrCodeThreadLocked)
	}
	if got := findThread(t, f.db, f.thread.ID).PostsCount; got != 1 {
		t.Errorf("thread.PostsCount = %d, want 1", got)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want 1", got)
	}
}
