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

// newEnableTwoFactorAuthUsecase builds an EnableTwoFactorAuthUsecase (and its
// validator) over the test's own database, creates a user, and seeds a
// not-yet-enabled enrollment with the default secret so a valid TOTP code can be
// generated for it. It returns the usecase, the repository (for assertions), the
// user ID, and a context.
//
// [Ja] newEnableTwoFactorAuthUsecase はテスト専用のデータベース上に
// EnableTwoFactorAuthUsecase (とその validator) を作り、ユーザーを作成し、既定の secret を
// 持つ未有効化の登録を投入して、それに対する有効な TOTP コードを生成できるようにする。
// usecase・(検証用の) リポジトリ・ユーザー ID・context を返す。
func newEnableTwoFactorAuthUsecase(t *testing.T, db *database.DB) (*usecase.EnableTwoFactorAuthUsecase, *repository.UserTwoFactorAuthRepository, model.UserID, context.Context) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()
	v := validator.NewSettingsTwoFactorAuthCreateValidator(repo)
	return usecase.NewEnableTwoFactorAuthUsecase(v, repo), repo, userID, context.Background()
}

// TestEnableTwoFactorAuthUsecase_Execute_Success verifies that a correct TOTP code
// enables the setting: it activates the row (enabled, enabled_at stamped), stores
// the generated recovery codes, and returns those same codes for one-time display.
//
// [Ja] TestEnableTwoFactorAuthUsecase_Execute_Success は、正しい TOTP コードが設定を有効化
// することを検証する。行をアクティブにし (enabled、enabled_at を打刻)、生成したリカバリー
// コードを保存し、一度だけ表示するためにそれと同じコードを返す。
func TestEnableTwoFactorAuthUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userID, ctx := newEnableTwoFactorAuthUsecase(t, db)

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: userID, Code: code})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(out.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Errorf("len(RecoveryCodes) = %d, want %d", len(out.RecoveryCodes), auth.RecoveryCodeCount)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("有効化後の設定が取得できない")
	}
	if !stored.Enabled {
		t.Error("有効化後の Enabled = false, want true")
	}
	if stored.EnabledAt == nil {
		t.Error("有効化後の EnabledAt が nil (打刻されていない)")
	}
	if len(stored.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Errorf("保存された len(RecoveryCodes) = %d, want %d", len(stored.RecoveryCodes), auth.RecoveryCodeCount)
	}
}

// TestEnableTwoFactorAuthUsecase_Execute_InvalidCode verifies that an incorrect code
// returns a ValidationError and leaves the setting not enabled.
//
// [Ja] TestEnableTwoFactorAuthUsecase_Execute_InvalidCode は、誤ったコードが
// ValidationError を返し、設定を未有効化のまま残すことを検証する。
func TestEnableTwoFactorAuthUsecase_Execute_InvalidCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userID, ctx := newEnableTwoFactorAuthUsecase(t, db)

	// A well-formed code deliberately not equal to the current one.
	//
	// [Ja] 整った形式で、意図的に現在のコードと等しくない値。
	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	_, err = uc.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: userID, Code: wrongCode})
	if model.AsValidationError(err) == nil {
		t.Fatalf("Execute() error = %v, want *ValidationError", err)
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("登録行が消えている")
	}
	if stored.Enabled {
		t.Error("誤ったコードで Enabled = true になった (有効化されるべきでない)")
	}
}
