package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newPrepareTwoFactorAuthUsecase builds a PrepareTwoFactorAuthUsecase bound to the
// test transaction and creates a user, returning the usecase, the repository (for
// assertions), the user ID, and a context. The usecase opens no transaction of its
// own, so the WithTx repository keeps every write inside the rolled-back test
// transaction.
//
// [Ja] newPrepareTwoFactorAuthUsecase はテスト用トランザクションに束ねた
// PrepareTwoFactorAuthUsecase を作りユーザーを作成して、usecase・(検証用の) リポジトリ・
// ユーザー ID・context を返す。usecase は自前のトランザクションを開かないため、WithTx
// リポジトリがすべての書き込みをロールバックされるテストトランザクション内に保つ。
func newPrepareTwoFactorAuthUsecase(t *testing.T) (*usecase.PrepareTwoFactorAuthUsecase, *repository.UserTwoFactorAuthRepository, model.UserID, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	repo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	return usecase.NewPrepareTwoFactorAuthUsecase(repo), repo, userID, context.Background()
}

// TestPrepareTwoFactorAuthUsecase_Execute_CreatesEnrollment verifies that with no
// existing setting, Execute generates a secret, persists a not-yet-enabled row, and
// returns that secret.
//
// [Ja] TestPrepareTwoFactorAuthUsecase_Execute_CreatesEnrollment は、既存の設定が無いとき
// Execute が secret を生成し、未有効化の行を永続化し、その secret を返すことを検証する。
func TestPrepareTwoFactorAuthUsecase_Execute_CreatesEnrollment(t *testing.T) {
	t.Parallel()

	uc, repo, userID, ctx := newPrepareTwoFactorAuthUsecase(t)

	out, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.AlreadyEnabled {
		t.Error("AlreadyEnabled = true, want false")
	}
	if out.Secret == "" {
		t.Error("Secret が空 (生成されていない)")
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("未有効化の登録行が作成されていない")
	}
	if stored.Enabled {
		t.Error("作成された行の Enabled = true, want false")
	}
	if stored.Secret != out.Secret {
		t.Errorf("保存された secret = %q, want %q (返り値と一致すべき)", stored.Secret, out.Secret)
	}
}

// TestPrepareTwoFactorAuthUsecase_Execute_ReusesInProgress verifies that an
// in-progress (not-yet-enabled) enrollment is reused: Execute returns its existing
// secret and does not create a second row.
//
// [Ja] TestPrepareTwoFactorAuthUsecase_Execute_ReusesInProgress は、登録中 (未有効化) の
// 設定が再利用されることを検証する。Execute は既存の secret を返し、2 つ目の行を作らない。
func TestPrepareTwoFactorAuthUsecase_Execute_ReusesInProgress(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	repo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewPrepareTwoFactorAuthUsecase(repo)
	ctx := context.Background()

	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(userID).Build()

	out, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.AlreadyEnabled {
		t.Error("AlreadyEnabled = true, want false")
	}
	if out.Secret != testutil.DefaultBuilderTOTPSecret {
		t.Errorf("Secret = %q, want %q (既存の登録の secret を再利用すべき)", out.Secret, testutil.DefaultBuilderTOTPSecret)
	}
}

// TestPrepareTwoFactorAuthUsecase_Execute_AlreadyEnabled verifies that when 2FA is
// already enabled, Execute reports AlreadyEnabled and returns no secret.
//
// [Ja] TestPrepareTwoFactorAuthUsecase_Execute_AlreadyEnabled は、2FA が既に有効なとき
// Execute が AlreadyEnabled を報告し secret を返さないことを検証する。
func TestPrepareTwoFactorAuthUsecase_Execute_AlreadyEnabled(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	repo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewPrepareTwoFactorAuthUsecase(repo)
	ctx := context.Background()

	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(userID).WithEnabled(true).Build()

	out, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !out.AlreadyEnabled {
		t.Error("AlreadyEnabled = false, want true")
	}
	if out.Secret != "" {
		t.Errorf("Secret = %q, want empty (既に有効なときは secret を返さない)", out.Secret)
	}
}
