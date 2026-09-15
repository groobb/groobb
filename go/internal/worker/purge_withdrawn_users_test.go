package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/worker"
)

// TestPurgeWithdrawnUsersWorker_Workは薄いAdapterがパージを駆動することを確認する。
// ジョブは引数を持たないためWorkはUseCaseを呼ぶだけである。保持期間より前に論理削除された
// ユーザーを用意し、Workがそれを物理削除することを検証する。実質的なカバレッジ (どのユーザーが
// 生き残るか・子行のCASCADE) はUseCaseテストにあり、本テストはWork経路がそこに配線されて
// いることを確認する。
func TestPurgeWithdrawnUsersWorker_Work(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(db)
	uc := usecase.NewPurgeWithdrawnUsersUsecase(userRepo)
	w := worker.NewPurgeWithdrawnUsersWorker(uc)

	oldWithdrawn := testutil.NewUserBuilder(t, db).
		WithDeletedAt(time.Now().Add(-60 * 24 * time.Hour)).
		Build()

	job := &river.Job[dispatcher.PurgeWithdrawnUsersArgs]{Args: dispatcher.PurgeWithdrawnUsersArgs{}}
	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	var exists bool
	if err := db.Writer.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`, int64(oldWithdrawn),
	).Scan(&exists); err != nil {
		t.Fatalf("ユーザー存在確認に失敗: %v", err)
	}
	if exists {
		t.Error("Workが退会済みユーザーの物理削除を駆動していない")
	}
}
