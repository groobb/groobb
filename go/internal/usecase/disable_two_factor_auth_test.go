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

// newDisableTwoFactorAuthUsecase builds a DisableTwoFactorAuthUsecase (and its
// validator) over the test's own database and creates a user, returning the
// usecase, the 2FA repository (for assertions), and the user ID so a test can seed
// the enabled setting and a password for that user.
//
// [Ja] newDisableTwoFactorAuthUsecase はテスト専用のデータベース上に
// DisableTwoFactorAuthUsecase (とその validator) を作り、ユーザーを作成して、usecase・
// (検証用の) 2FA リポジトリ・(有効な設定とパスワードを投入するための) ユーザー ID を返す。
func newDisableTwoFactorAuthUsecase(t *testing.T, db *database.DB) (*usecase.DisableTwoFactorAuthUsecase, *repository.UserTwoFactorAuthRepository, model.UserID) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	v := validator.NewSettingsTwoFactorAuthDeleteValidator(userPasswordRepo, repo)
	return usecase.NewDisableTwoFactorAuthUsecase(v, repo), repo, userID
}

// TestDisableTwoFactorAuthUsecase_Execute_Success verifies that a correct current
// password disables 2FA: the setting row is deleted, discarding the secret and
// recovery codes with it.
//
// [Ja] TestDisableTwoFactorAuthUsecase_Execute_Success は、正しい現在のパスワードが 2FA を
// 無効化することを検証する。設定行が削除され、secret とリカバリーコードが行ごと破棄される。
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
		t.Fatalf("Execute() error = %v", err)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored != nil {
		t.Error("無効化後も 2FA 設定が残っている")
	}
}

// TestDisableTwoFactorAuthUsecase_Execute_InvalidReauth verifies that a wrong current
// password (and no code) returns a ValidationError and leaves 2FA enabled.
//
// [Ja] TestDisableTwoFactorAuthUsecase_Execute_InvalidReauth は、誤った現在のパスワード
// (コードなし) が ValidationError を返し、2FA を有効なまま残すことを検証する。
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
		t.Fatalf("Execute() error = %v, want *ValidationError", err)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("バリデーション失敗時に 2FA 設定が削除された")
	}
	if !stored.Enabled {
		t.Error("バリデーション失敗時に 2FA が無効化された")
	}
}
