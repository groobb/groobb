package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// EnableTwoFactorAuthUsecaseは2段階認証の有効化を統括します。送信されたTOTPコードを
// 登録中の設定のsecretに対して検証し、1回使い切りのリカバリーコードを生成し、設定を
// アクティブにします (enabledにし、enabled_atを打刻し、リカバリーコードを保存する)。
// リカバリーコードはハンドラーが一度だけ表示できるよう返します。二度と表示されません。
// 有効化は軽いインメモリ処理 (コード生成) を伴う単一のUPDATEのため、トランザクションは
// 取りません。
type EnableTwoFactorAuthUsecase struct {
	validator             *validator.SettingsTwoFactorAuthCreateValidator
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewEnableTwoFactorAuthUsecaseはvalidatorと2FAリポジトリから
// EnableTwoFactorAuthUsecaseを構築します。
func NewEnableTwoFactorAuthUsecase(
	validator *validator.SettingsTwoFactorAuthCreateValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *EnableTwoFactorAuthUsecase {
	return &EnableTwoFactorAuthUsecase{
		validator:             validator,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
	}
}

// EnableTwoFactorAuthInputはExecuteの入力です。UserIDは2FAを有効化するサインイン
// 済みユーザー (セッションで確定する)、Codeはユーザーが認証アプリから入力したTOTPコード
// です。
type EnableTwoFactorAuthInput struct {
	UserID model.UserID
	Code   string
}

// EnableTwoFactorAuthOutputは生成したリカバリーコードを運び、ハンドラーが一度だけ
// 表示できるようにします。これらは設定に平文で保存され、ユーザーが控えられる唯一の機会の
// ため、ハンドラーは成功時に必ず描画しなければなりません。
type EnableTwoFactorAuthOutput struct {
	RecoveryCodes []string
}

// Executeはコードを検証し、リカバリーコードを生成し、設定をアクティブにします。
// バリデーション (形式 + 保存済みsecretに対するコード検証) を先に走らせ、誤った / 不正な
// コードや登録の不在ではアクティブにせずに返します。リカバリーコードは書き込みの前に生成し、
// 単一の永続化呼び出し (Enable) を唯一の副作用に保ちます。
//
// Enableはenabled = falseでガードされているため、バリデーションからこの書き込みまでの間に
// 同時のリクエストが2FAを有効化した場合 (同一コードの二重送信)、1行も有効化せずfalseを
// 返します。その場合、生成したてのこのコードは保存されていないため表示してはなりません。
// validatorが「設定が失われた / 既に有効」で出すのと同じフォーム全体のValidationErrorを
// 返し、ハンドラーはこれを使えないコードのページではなく設定ハブへのリダイレクトに変えます。
func (uc *EnableTwoFactorAuthUsecase) Execute(ctx context.Context, input EnableTwoFactorAuthInput) (*EnableTwoFactorAuthOutput, error) {
	if err := uc.validator.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{
		UserID: input.UserID,
		Code:   input.Code,
	}); err != nil {
		return nil, err
	}

	recoveryCodes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		return nil, fmt.Errorf("リカバリーコードの生成に失敗: %w", err)
	}

	enabled, err := uc.userTwoFactorAuthRepo.Enable(ctx, input.UserID, recoveryCodes)
	if err != nil {
		return nil, fmt.Errorf("2段階認証の有効化に失敗: %w", err)
	}
	if !enabled {
		// 同時の有効化が競合に勝ち (enabled = falseガードが行に一致せず)、
		// recoveryCodesは保存されなかった。設定が失われたと報告してハンドラーに再描画・
		// ハブへのリダイレクトをさせ、保存されていないコードを表示させない。
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "validation_totp_setup_invalid"))
		return nil, ve
	}

	return &EnableTwoFactorAuthOutput{RecoveryCodes: recoveryCodes}, nil
}
