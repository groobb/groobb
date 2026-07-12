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

// EnableTwoFactorAuthUsecase orchestrates enabling two-factor authentication: it
// validates the submitted TOTP code against the in-progress enrollment's secret,
// generates the one-time recovery codes, and activates the setting (marks it
// enabled, stamps enabled_at, and stores the recovery codes). The recovery codes
// are returned so the handler can show them once; they are never displayed again.
// It takes no transaction because activation is a single UPDATE preceded by cheap
// in-memory work (code generation).
//
// [Ja] EnableTwoFactorAuthUsecase は 2 段階認証の有効化を統括します。送信された TOTP コードを
// 登録中の設定の secret に対して検証し、1 回使い切りのリカバリーコードを生成し、設定を
// アクティブにします (enabled にし、enabled_at を打刻し、リカバリーコードを保存する)。
// リカバリーコードはハンドラーが一度だけ表示できるよう返します。二度と表示されません。
// 有効化は軽いインメモリ処理 (コード生成) を伴う単一の UPDATE のため、トランザクションは
// 取りません。
type EnableTwoFactorAuthUsecase struct {
	validator             *validator.SettingsTwoFactorAuthCreateValidator
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewEnableTwoFactorAuthUsecase builds an EnableTwoFactorAuthUsecase from its
// validator and the 2FA repository.
//
// [Ja] NewEnableTwoFactorAuthUsecase は validator と 2FA リポジトリから
// EnableTwoFactorAuthUsecase を構築します。
func NewEnableTwoFactorAuthUsecase(
	validator *validator.SettingsTwoFactorAuthCreateValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *EnableTwoFactorAuthUsecase {
	return &EnableTwoFactorAuthUsecase{
		validator:             validator,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
	}
}

// EnableTwoFactorAuthInput is the input to Execute. UserID is the signed-in user
// enabling 2FA (established by the session), and Code is the TOTP code the user
// typed from their authenticator app.
//
// [Ja] EnableTwoFactorAuthInput は Execute の入力です。UserID は 2FA を有効化するサインイン
// 済みユーザー (セッションで確定する)、Code はユーザーが認証アプリから入力した TOTP コード
// です。
type EnableTwoFactorAuthInput struct {
	UserID model.UserID
	Code   string
}

// EnableTwoFactorAuthOutput carries the generated recovery codes so the handler can
// show them once. They are stored in plaintext on the setting and are the only
// chance the user has to record them, so the handler must render them on success.
//
// [Ja] EnableTwoFactorAuthOutput は生成したリカバリーコードを運び、ハンドラーが一度だけ
// 表示できるようにします。これらは設定に平文で保存され、ユーザーが控えられる唯一の機会の
// ため、ハンドラーは成功時に必ず描画しなければなりません。
type EnableTwoFactorAuthOutput struct {
	RecoveryCodes []string
}

// Execute validates the code, generates the recovery codes, and activates the
// setting. Validation (format + code verification against the stored secret) runs
// first, so a wrong or malformed code, or a missing enrollment, returns without
// activating. The recovery codes are generated before the write, keeping the single
// persistence call (Enable) as the only side effect.
//
// Enable is guarded by enabled = false, so if a concurrent request enabled 2FA
// between validation and this write (a double submit of the same code), it enables
// no row and reports false. In that case these freshly generated codes were never
// stored, so they must not be shown: it returns the same form-wide ValidationError
// the validator raises for a gone / already-enabled setup, which the handler turns
// into a redirect to the settings hub rather than a page of unusable codes.
//
// [Ja] Execute はコードを検証し、リカバリーコードを生成し、設定をアクティブにします。
// バリデーション (形式 + 保存済み secret に対するコード検証) を先に走らせ、誤った / 不正な
// コードや登録の不在ではアクティブにせずに返します。リカバリーコードは書き込みの前に生成し、
// 単一の永続化呼び出し (Enable) を唯一の副作用に保ちます。
//
// Enable は enabled = false でガードされているため、バリデーションからこの書き込みまでの間に
// 同時のリクエストが 2FA を有効化した場合 (同一コードの二重送信)、1 行も有効化せず false を
// 返します。その場合、生成したてのこのコードは保存されていないため表示してはなりません。
// validator が「設定が失われた / 既に有効」で出すのと同じフォーム全体の ValidationError を
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
		return nil, fmt.Errorf("2 段階認証の有効化に失敗: %w", err)
	}
	if !enabled {
		// A concurrent enable won the race (the enabled = false guard matched no
		// row), so recoveryCodes were not stored. Report the setup as gone so the
		// handler re-renders and redirects to the hub, never showing unstored codes.
		//
		// [Ja] 同時の有効化が競合に勝ち (enabled = false ガードが行に一致せず)、
		// recoveryCodes は保存されなかった。設定が失われたと報告してハンドラーに再描画・
		// ハブへのリダイレクトをさせ、保存されていないコードを表示させない。
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "validation_totp_setup_invalid"))
		return nil, ve
	}

	return &EnableTwoFactorAuthOutput{RecoveryCodes: recoveryCodes}, nil
}
