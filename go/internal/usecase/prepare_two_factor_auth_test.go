package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newPrepareTwoFactorAuthUsecaseはテスト専用のデータベース上に
// PrepareTwoFactorAuthUsecaseを作りユーザーを作成して、usecase・(検証用の) リポジトリ・
// ユーザーID・contextを返す。
func newPrepareTwoFactorAuthUsecase(t *testing.T, db *database.DB) (*usecase.PrepareTwoFactorAuthUsecase, *repository.UserTwoFactorAuthRepository, model.UserID, context.Context) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	return usecase.NewPrepareTwoFactorAuthUsecase(repo), repo, userID, context.Background()
}

// TestPrepareTwoFactorAuthUsecase_Execute_CreatesEnrollmentは、既存の設定が無いとき
// Executeがsecretを生成し、未有効化の行を永続化し、そのsecretを返すことを検証する。
func TestPrepareTwoFactorAuthUsecase_Execute_CreatesEnrollment(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userID, ctx := newPrepareTwoFactorAuthUsecase(t, db)

	out, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out.AlreadyEnabled {
		t.Error("AlreadyEnabled = true、期待値 = false")
	}
	if out.Secret == "" {
		t.Error("Secretが空 (生成されていない)")
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("未有効化の登録行が作成されていない")
	}
	if stored.Enabled {
		t.Error("作成された行のEnabled = true、期待値 = false")
	}
	if stored.Secret != out.Secret {
		t.Errorf("保存されたsecret = %q、期待値 = %q (返り値と一致すべき)", stored.Secret, out.Secret)
	}
}

// TestPrepareTwoFactorAuthUsecase_Execute_ReusesInProgressは、登録中 (未有効化) の
// 設定が再利用されることを検証する。Executeは既存のsecretを返し、2つ目の行を作らない。
func TestPrepareTwoFactorAuthUsecase_Execute_ReusesInProgress(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	uc := usecase.NewPrepareTwoFactorAuthUsecase(repo)
	ctx := context.Background()

	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

	out, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out.AlreadyEnabled {
		t.Error("AlreadyEnabled = true、期待値 = false")
	}
	if out.Secret != testutil.DefaultBuilderTOTPSecret {
		t.Errorf("Secret = %q、期待値 = %q (既存の登録のsecretを再利用すべき)", out.Secret, testutil.DefaultBuilderTOTPSecret)
	}
}

// TestPrepareTwoFactorAuthUsecase_Execute_AlreadyEnabledは、2FAが既に有効なとき
// ExecuteがAlreadyEnabledを報告しsecretを返さないことを検証する。
func TestPrepareTwoFactorAuthUsecase_Execute_AlreadyEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	uc := usecase.NewPrepareTwoFactorAuthUsecase(repo)
	ctx := context.Background()

	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

	out, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if !out.AlreadyEnabled {
		t.Error("AlreadyEnabled = false、期待値 = true")
	}
	if out.Secret != "" {
		t.Errorf("Secret = %q、期待値 = 空 (既に有効なときはsecretを返さない)", out.Secret)
	}
}
