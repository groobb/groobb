package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// DisableTwoFactorAuthUsecaseは2段階認証の無効化を統括します。リクエストを
// 再認証し (現在のパスワードか現在のTOTPコード)、成功時にユーザーの2FA設定を削除して
// secretとリカバリーコードを行ごと破棄します。削除は冪等なため、設定が既に無い場合 (例:
// 別タブで同時に無効化された) もエラーになりません。無効化はバリデーションを伴うだけの単一の
// 永続化呼び出しのため、トランザクションは取りません。
type DisableTwoFactorAuthUsecase struct {
	validator             *validator.SettingsTwoFactorAuthDeleteValidator
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewDisableTwoFactorAuthUsecaseはvalidatorと2FAリポジトリから
// DisableTwoFactorAuthUsecaseを構築します。
func NewDisableTwoFactorAuthUsecase(
	validator *validator.SettingsTwoFactorAuthDeleteValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *DisableTwoFactorAuthUsecase {
	return &DisableTwoFactorAuthUsecase{
		validator:             validator,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
	}
}

// DisableTwoFactorAuthInputはExecuteの入力です。UserIDは2FAを無効化する
// サインイン済みユーザー (セッションで確定する) です。CurrentPasswordとCodeはユーザーが
// 送信した再認証の値で、どちらか一方を使います。
type DisableTwoFactorAuthInput struct {
	UserID          model.UserID
	CurrentPassword string
	Code            string
}

// Executeは再認証してから2FAを無効化します。バリデーションを先に走らせるため、
// 未入力 / 誤った再認証では行に触れず *model.ValidationErrorを返します。成功時に設定を
// 削除します。削除は冪等なため、設定が既に無い場合もエラーになりません。バリデーションを
// 伴うだけの単一の永続化呼び出しのため、Execute内に置き、トランザクションは不要です。
func (uc *DisableTwoFactorAuthUsecase) Execute(ctx context.Context, input DisableTwoFactorAuthInput) error {
	if err := uc.validator.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{
		UserID:          input.UserID,
		CurrentPassword: input.CurrentPassword,
		Code:            input.Code,
	}); err != nil {
		return err
	}

	if err := uc.userTwoFactorAuthRepo.Delete(ctx, input.UserID); err != nil {
		return fmt.Errorf("2段階認証の無効化に失敗: %w", err)
	}
	return nil
}
