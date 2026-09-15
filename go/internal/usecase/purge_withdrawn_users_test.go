package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// userExistsは指定idのusers行がまだ存在するかを返す。パージテストがどのユーザーが
// 生き残ったかを検証できるようにする。
func userExists(t *testing.T, db *database.DB, id model.UserID) bool {
	t.Helper()

	var exists bool
	if err := db.Writer.QueryRowContext(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`, int64(id),
	).Scan(&exists); err != nil {
		t.Fatalf("ユーザー存在確認に失敗: %v", err)
	}
	return exists
}

// sessionCountは指定ユーザーが所有するセッション数を返す。パージが退会ユーザーの
// 子行をON DELETE CASCADEで削除したことを検証するために使う。
func sessionCount(t *testing.T, db *database.DB, userID model.UserID) int {
	t.Helper()

	var count int
	if err := db.Writer.QueryRowContext(context.Background(),
		`SELECT count(*) FROM user_sessions WHERE user_id = ?`, int64(userID),
	).Scan(&count); err != nil {
		t.Fatalf("セッション件数の取得に失敗: %v", err)
	}
	return count
}

// TestPurgeWithdrawnUsersUsecase_Executeは、保持期間より前に論理削除された
// ユーザーを (その子行もCASCADEで) 物理削除する一方、最近退会したユーザーとアクティブな
// ユーザーには手を付けないことを検証する。
func TestPurgeWithdrawnUsersUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(db)
	uc := usecase.NewPurgeWithdrawnUsersUsecase(userRepo)

	now := time.Now()
	// 30日の保持期間より十分前に論理削除済み: 物理削除されるべき。
	oldWithdrawn := testutil.NewUserBuilder(t, db).WithDeletedAt(now.Add(-60 * 24 * time.Hour)).Build()
	// 直前に論理削除済みで保持期間内: 生き残るべき。
	recentWithdrawn := testutil.NewUserBuilder(t, db).WithDeletedAt(now.Add(-time.Hour)).Build()
	// 退会していない: 生き残るべき。
	active := testutil.NewUserBuilder(t, db).Build()

	// パージ対象ユーザーにセッションを持たせ、子行がON DELETE CASCADEで一緒に
	// 消えることも検証する。
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO user_sessions (user_id, token, ip_address, user_agent) VALUES (?, ?, ?, ?)`,
		int64(oldWithdrawn), "purge-token", "127.0.0.1", "test-agent",
	); err != nil {
		t.Fatalf("テスト用セッションの作成に失敗: %v", err)
	}

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	if userExists(t, db, oldWithdrawn) {
		t.Error("猶予期間を過ぎた退会ユーザーが物理削除されていない")
	}
	if got := sessionCount(t, db, oldWithdrawn); got != 0 {
		t.Errorf("退会ユーザーの子データ (セッション) 数 = %d、期待値 = 0 (CASCADEで削除されるべき)", got)
	}
	if !userExists(t, db, recentWithdrawn) {
		t.Error("猶予期間内の退会ユーザーが誤って削除された")
	}
	if !userExists(t, db, active) {
		t.Error("アクティブなユーザーが誤って削除された")
	}
}
