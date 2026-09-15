package validator

import (
	"context"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SettingsEmailConfirmationCreateValidatorは送信されたメール変更の確認コードの
// 形式を検証し、サインイン済みユーザーのまだ有効なメール変更の確認を解決します。
// サインアップのEmailConfirmationCreateValidatorと対をなしますが、受け渡しCookieが
// 運ぶ確認idではなく (セッションから得る) ユーザーidをキーにします。メール変更フローは
// 認証済みユーザーから保留中の確認を特定するためです。コードを保存値と照合はしません。
// 不一致は確認の失敗試行回数をインクリメント (DB書き込み) する必要があり、バリデーション
// ガイドラインに従いコード照合とそのインクリメントはUseCaseのトランザクションに置くため
// です。ここに残る検証 (形式、およびアクティブな確認が存在するか) はいずれも書き込まない
// ためvalidatorに属します。
type SettingsEmailConfirmationCreateValidator struct {
	emailConfirmationRepo *repository.EmailConfirmationRepository
}

// NewSettingsEmailConfirmationCreateValidatorは
// SettingsEmailConfirmationCreateValidatorを生成します。
func NewSettingsEmailConfirmationCreateValidator(emailConfirmationRepo *repository.EmailConfirmationRepository) *SettingsEmailConfirmationCreateValidator {
	return &SettingsEmailConfirmationCreateValidator{emailConfirmationRepo: emailConfirmationRepo}
}

// SettingsEmailConfirmationCreateValidatorInputはValidateの入力です。UserIDは
// コードを送信するサインイン済みユーザー (セッションで確定する)、Codeはユーザーが
// 入力した値です。
type SettingsEmailConfirmationCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validateはコードの形式を検証し、ユーザーのまだ有効なメール変更の確認を返して、
// UseCaseが再クエリせずにコードを照合し打刻または試行回数の計上をできるようにします。
// 形式の問題はcodeフィールドに付け、確認が無い / 期限切れ / 試行回数超過 (いずれも
// FindActiveEmailChangeByUserIDがnilとして報告する) はフォーム全体のメッセージ1件と
// して返します。このnilのケースとUseCase側のコード不一致のケースを意図的に同じ
// メッセージにまとめ、実在の確認が保留中だったかをフォームが漏らさないようにします。
// コード自体はここでは照合しません。不一致は書き込み (試行回数のインクリメント) を要する
// ため、照合はUseCaseのトランザクションに属します。本物のクエリ失敗は素のerrorとして
// 表れます (上流で500として扱う)。
func (v *SettingsEmailConfirmationCreateValidator) Validate(ctx context.Context, input SettingsEmailConfirmationCreateValidatorInput) (*model.EmailConfirmation, error) {
	ve := model.NewValidationError()

	if input.Code == "" {
		ve.AddField("code", i18n.T(ctx, "validation_required"))
		return nil, ve
	}

	if !confirmationCodeRegex.MatchString(input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_code_invalid_format"))
		return nil, ve
	}

	confirmation, err := v.emailConfirmationRepo.FindActiveEmailChangeByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if confirmation == nil {
		ve.AddGlobal(i18n.T(ctx, "validation_code_incorrect_or_expired"))
		return nil, ve
	}

	return confirmation, nil
}
