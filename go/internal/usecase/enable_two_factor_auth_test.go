package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newEnableTwoFactorAuthUsecaseはテスト専用のデータベース上に
// EnableTwoFactorAuthUsecase (とそのvalidator) を作り、ユーザーを作成し、既定のsecretを
// 持つ未有効化の登録を投入して、それに対する有効なTOTPコードを生成できるようにする。
// usecase・(検証用の) リポジトリ・ユーザーID・contextを返す。
func newEnableTwoFactorAuthUsecase(t *testing.T, db *database.DB) (*usecase.EnableTwoFactorAuthUsecase, *repository.UserTwoFactorAuthRepository, model.UserID, context.Context) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()
	v := validator.NewSettingsTwoFactorAuthCreateValidator(repo)
	return usecase.NewEnableTwoFactorAuthUsecase(v, repo), repo, userID, context.Background()
}

// TestEnableTwoFactorAuthUsecase_Execute_Successは、正しいTOTPコードが設定を有効化
// することを検証する。行をアクティブにし (enabled、enabled_atを打刻)、生成したリカバリー
// コードを保存し、一度だけ表示するためにそれと同じコードを返す。
func TestEnableTwoFactorAuthUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userID, ctx := newEnableTwoFactorAuthUsecase(t, db)

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: userID, Code: code})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(out.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Errorf("len(RecoveryCodes) = %d、期待値 = %d", len(out.RecoveryCodes), auth.RecoveryCodeCount)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("有効化後の設定が取得できない")
	}
	if !stored.Enabled {
		t.Error("有効化後のEnabled = false、期待値 = true")
	}
	if stored.EnabledAt == nil {
		t.Error("有効化後のEnabledAtがnil (打刻されていない)")
	}
	if len(stored.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Errorf("保存されたlen(RecoveryCodes) = %d、期待値 = %d", len(stored.RecoveryCodes), auth.RecoveryCodeCount)
	}
}

// TestEnableTwoFactorAuthUsecase_Execute_InvalidCodeは、誤ったコードが
// ValidationErrorを返し、設定を未有効化のまま残すことを検証する。
func TestEnableTwoFactorAuthUsecase_Execute_InvalidCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userID, ctx := newEnableTwoFactorAuthUsecase(t, db)

	// 整った形式で、意図的に現在のコードと等しくない値。
	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	_, err = uc.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: userID, Code: wrongCode})
	if model.AsValidationError(err) == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *ValidationError", err)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("登録行が消えている")
	}
	if stored.Enabled {
		t.Error("誤ったコードでEnabled = trueになった (有効化されるべきでない)")
	}
}
