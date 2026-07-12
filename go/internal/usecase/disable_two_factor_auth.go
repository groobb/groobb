package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// DisableTwoFactorAuthUsecase orchestrates disabling two-factor authentication: it
// re-authenticates the request (the current password or a current TOTP code) and, on
// success, deletes the user's 2FA setting, discarding the secret and recovery codes
// with the row. Deleting is idempotent, so a setting already gone (e.g. disabled
// concurrently in another tab) is not an error. It takes no transaction because the
// delete is a single persistence call preceded only by validation.
//
// [Ja] DisableTwoFactorAuthUsecase は 2 段階認証の無効化を統括します。リクエストを
// 再認証し (現在のパスワードか現在の TOTP コード)、成功時にユーザーの 2FA 設定を削除して
// secret とリカバリーコードを行ごと破棄します。削除は冪等なため、設定が既に無い場合 (例:
// 別タブで同時に無効化された) もエラーになりません。無効化はバリデーションを伴うだけの単一の
// 永続化呼び出しのため、トランザクションは取りません。
type DisableTwoFactorAuthUsecase struct {
	validator             *validator.SettingsTwoFactorAuthDeleteValidator
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewDisableTwoFactorAuthUsecase builds a DisableTwoFactorAuthUsecase from its
// validator and the 2FA repository.
//
// [Ja] NewDisableTwoFactorAuthUsecase は validator と 2FA リポジトリから
// DisableTwoFactorAuthUsecase を構築します。
func NewDisableTwoFactorAuthUsecase(
	validator *validator.SettingsTwoFactorAuthDeleteValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *DisableTwoFactorAuthUsecase {
	return &DisableTwoFactorAuthUsecase{
		validator:             validator,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
	}
}

// DisableTwoFactorAuthInput is the input to Execute. UserID is the signed-in user
// disabling 2FA (established by the session). CurrentPassword and Code are the
// re-authentication values the user submitted; one of the two is used.
//
// [Ja] DisableTwoFactorAuthInput は Execute の入力です。UserID は 2FA を無効化する
// サインイン済みユーザー (セッションで確定する) です。CurrentPassword と Code はユーザーが
// 送信した再認証の値で、どちらか一方を使います。
type DisableTwoFactorAuthInput struct {
	UserID          model.UserID
	CurrentPassword string
	Code            string
}

// Execute re-authenticates and then disables 2FA. Validation runs first, so a
// missing or incorrect re-authentication returns a *model.ValidationError without
// touching the row. On success it deletes the setting; the delete is idempotent, so
// a setting already gone is not an error. It is a single persistence call preceded
// only by validation, so it stays in Execute and needs no transaction.
//
// [Ja] Execute は再認証してから 2FA を無効化します。バリデーションを先に走らせるため、
// 未入力 / 誤った再認証では行に触れず *model.ValidationError を返します。成功時に設定を
// 削除します。削除は冪等なため、設定が既に無い場合もエラーになりません。バリデーションを
// 伴うだけの単一の永続化呼び出しのため、Execute 内に置き、トランザクションは不要です。
func (uc *DisableTwoFactorAuthUsecase) Execute(ctx context.Context, input DisableTwoFactorAuthInput) error {
	if err := uc.validator.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{
		UserID:          input.UserID,
		CurrentPassword: input.CurrentPassword,
		Code:            input.Code,
	}); err != nil {
		return err
	}

	if err := uc.userTwoFactorAuthRepo.Delete(ctx, input.UserID); err != nil {
		return fmt.Errorf("2 段階認証の無効化に失敗: %w", err)
	}
	return nil
}
