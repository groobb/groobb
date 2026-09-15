package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newDisableTwoFactorAuthUsecaseはテスト専用のデータベース上に
// DisableTwoFactorAuthUsecase (とそのvalidator) を作り、ユーザーを作成して、usecase・
// (検証用の) 2FAリポジトリ・(有効な設定とパスワードを投入するための) ユーザーIDを返す。
func newDisableTwoFactorAuthUsecase(t *testing.T, db *database.DB) (*usecase.DisableTwoFactorAuthUsecase, *repository.UserTwoFactorAuthRepository, model.UserID) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	v := validator.NewSettingsTwoFactorAuthDeleteValidator(userPasswordRepo, repo)
	return usecase.NewDisableTwoFactorAuthUsecase(v, repo), repo, userID
}

// TestDisableTwoFactorAuthUsecase_Execute_Successは、正しい現在のパスワードが2FAを
// 無効化することを検証する。設定行が削除され、secretとリカバリーコードが行ごと破棄される。
func TestDisableTwoFactorAuthUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userID := newDisableTwoFactorAuthUsecase(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).Build()

	ctx := context.Background()
	if err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{
		UserID:          userID,
		CurrentPassword: testutil.DefaultBuilderPassword,
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored != nil {
		t.Error("無効化後も2FA設定が残っている")
	}
}

// TestDisableTwoFactorAuthUsecase_Execute_InvalidReauthは、誤った現在のパスワード
// (コードなし) がValidationErrorを返し、2FAを有効なまま残すことを検証する。
func TestDisableTwoFactorAuthUsecase_Execute_InvalidReauth(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userID := newDisableTwoFactorAuthUsecase(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).Build()

	ctx := context.Background()
	err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{
		UserID:          userID,
		CurrentPassword: "wrongpassword",
	})
	if model.AsValidationError(err) == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *ValidationError", err)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("バリデーション失敗時に2FA設定が削除された")
	}
	if !stored.Enabled {
		t.Error("バリデーション失敗時に2FAが無効化された")
	}
}
