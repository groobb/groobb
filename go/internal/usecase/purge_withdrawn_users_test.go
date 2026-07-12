package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// userExists reports whether a users row with the given id still exists, so a purge
// test can assert which users survived.
//
// [Ja] userExists は指定 id の users 行がまだ存在するかを返す。パージテストがどのユーザーが
// 生き残ったかを検証できるようにする。
func userExists(t *testing.T, tx pgx.Tx, id model.UserID) bool {
	t.Helper()

	var exists bool
	if err := tx.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, uuid.UUID(id),
	).Scan(&exists); err != nil {
		t.Fatalf("ユーザー存在確認に失敗: %v", err)
	}
	return exists
}

// sessionCount returns how many sessions the given user owns, for asserting the
// purge removed a withdrawn user's child rows via ON DELETE CASCADE.
//
// [Ja] sessionCount は指定ユーザーが所有するセッション数を返す。パージが退会ユーザーの
// 子行を ON DELETE CASCADE で削除したことを検証するために使う。
func sessionCount(t *testing.T, tx pgx.Tx, userID model.UserID) int {
	t.Helper()

	var count int
	if err := tx.QueryRow(context.Background(),
		`SELECT count(*) FROM user_sessions WHERE user_id = $1`, uuid.UUID(userID),
	).Scan(&count); err != nil {
		t.Fatalf("セッション件数の取得に失敗: %v", err)
	}
	return count
}

// TestPurgeWithdrawnUsersUsecase_Execute verifies that the purge physically deletes
// a user soft-deleted before the retention window (along with its child rows via
// CASCADE), while leaving a recently withdrawn user and an active user untouched.
// The UseCase opens no transaction, so the test uses the rolled-back SetupTx pattern
// and drives the purge repository over that same transaction.
//
// [Ja] TestPurgeWithdrawnUsersUsecase_Execute は、保持期間より前に論理削除された
// ユーザーを (その子行も CASCADE で) 物理削除する一方、最近退会したユーザーとアクティブな
// ユーザーには手を付けないことを検証する。UseCase はトランザクションを開かないため、テストは
// ロールバックされる SetupTx パターンを使い、同じトランザクション上でパージリポジトリを駆動する。
func TestPurgeWithdrawnUsersUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewPurgeWithdrawnUsersUsecase(userRepo)

	now := time.Now()
	// Soft-deleted well before the 30-day retention window: must be purged.
	//
	// [Ja] 30 日の保持期間より十分前に論理削除済み: 物理削除されるべき。
	oldWithdrawn := testutil.NewUserBuilder(t, tx).WithDeletedAt(now.Add(-60 * 24 * time.Hour)).Build()
	// Soft-deleted just now, well within the window: must survive.
	//
	// [Ja] 直前に論理削除済みで保持期間内: 生き残るべき。
	recentWithdrawn := testutil.NewUserBuilder(t, tx).WithDeletedAt(now.Add(-time.Hour)).Build()
	// Never withdrawn: must survive.
	//
	// [Ja] 退会していない: 生き残るべき。
	active := testutil.NewUserBuilder(t, tx).Build()

	// Give the purge-eligible user a session so the test also proves child rows go
	// with it via ON DELETE CASCADE.
	//
	// [Ja] パージ対象ユーザーにセッションを持たせ、子行が ON DELETE CASCADE で一緒に
	// 消えることも検証する。
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_sessions (user_id, token, ip_address, user_agent) VALUES ($1, $2, $3, $4)`,
		uuid.UUID(oldWithdrawn), "purge-token-"+uuid.NewString(), "127.0.0.1", "test-agent",
	); err != nil {
		t.Fatalf("テスト用セッションの作成に失敗: %v", err)
	}

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if userExists(t, tx, oldWithdrawn) {
		t.Error("猶予期間を過ぎた退会ユーザーが物理削除されていない")
	}
	if got := sessionCount(t, tx, oldWithdrawn); got != 0 {
		t.Errorf("退会ユーザーの子データ (セッション) 数 = %d, want 0 (CASCADE で削除されるべき)", got)
	}
	if !userExists(t, tx, recentWithdrawn) {
		t.Error("猶予期間内の退会ユーザーが誤って削除された")
	}
	if !userExists(t, tx, active) {
		t.Error("アクティブなユーザーが誤って削除された")
	}
}
