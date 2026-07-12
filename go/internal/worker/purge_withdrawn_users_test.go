package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/worker"
)

// TestPurgeWithdrawnUsersWorker_Work checks the thin adapter drives the purge: the
// job carries no arguments, so Work just calls the UseCase. It seeds a user
// soft-deleted before the retention window and asserts Work physically deletes it.
// The substantive coverage (which users survive, CASCADE of child rows) lives in
// the UseCase test; this confirms the Work path is wired to it.
//
// [Ja] TestPurgeWithdrawnUsersWorker_Work は薄い Adapter がパージを駆動することを確認する。
// ジョブは引数を持たないため Work は UseCase を呼ぶだけである。保持期間より前に論理削除された
// ユーザーを用意し、Work がそれを物理削除することを検証する。実質的なカバレッジ (どのユーザーが
// 生き残るか・子行の CASCADE) は UseCase テストにあり、本テストは Work 経路がそこに配線されて
// いることを確認する。
func TestPurgeWithdrawnUsersWorker_Work(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewPurgeWithdrawnUsersUsecase(userRepo)
	w := worker.NewPurgeWithdrawnUsersWorker(uc)

	oldWithdrawn := testutil.NewUserBuilder(t, tx).
		WithDeletedAt(time.Now().Add(-60 * 24 * time.Hour)).
		Build()

	job := &river.Job[dispatcher.PurgeWithdrawnUsersArgs]{Args: dispatcher.PurgeWithdrawnUsersArgs{}}
	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, uuid.UUID(oldWithdrawn),
	).Scan(&exists); err != nil {
		t.Fatalf("ユーザー存在確認に失敗: %v", err)
	}
	if exists {
		t.Error("Work が退会済みユーザーの物理削除を駆動していない")
	}
}
