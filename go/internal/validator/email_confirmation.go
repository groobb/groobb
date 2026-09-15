package validator

import (
	"context"
	"regexp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// confirmationCodeRegexは6桁の数字確認コード (GenerateConfirmationCodeが
// 発行する形式) にマッチします。リクエストごとの検証コストをゼロにするため、パッケージ
// レベルで1度だけコンパイルします。
var confirmationCodeRegex = regexp.MustCompile(`^\d{6}$`)

// EmailConfirmationCreateValidatorは送信された確認コードの形式を検証し、
// サインアップから運ばれたidのまだ有効な確認を解決します。コードを保存値と照合は
// しません。不一致は確認の失敗試行回数をインクリメント (DB書き込み) する必要があり、
// バリデーションガイドラインに従いコード照合とそのインクリメントはUseCaseの
// トランザクションに置くためです。ここに残る検証 (形式、およびアクティブな確認が
// 存在するか) はいずれも書き込まないためvalidatorに属します。
type EmailConfirmationCreateValidator struct {
	emailConfirmationRepo *repository.EmailConfirmationRepository
}

// NewEmailConfirmationCreateValidatorはEmailConfirmationCreateValidatorを
// 生成します。
func NewEmailConfirmationCreateValidator(emailConfirmationRepo *repository.EmailConfirmationRepository) *EmailConfirmationCreateValidator {
	return &EmailConfirmationCreateValidator{emailConfirmationRepo: emailConfirmationRepo}
}

// EmailConfirmationCreateValidatorInputはValidateの入力です。IDは保留中の
// 確認のid (ハンドラーが受け渡しCookieから読む)、Codeはユーザーが入力した値です。
type EmailConfirmationCreateValidatorInput struct {
	ID   model.EmailConfirmationID
	Code string
}

// Validateはコードの形式を検証し、idのまだ有効な確認を返して、UseCaseが再クエリ
// せずにコードを照合し打刻または試行回数の計上をできるようにします。形式の問題はcode
// フィールドに付け、確認が無い / 期限切れ / 試行回数超過 (いずれもFindActiveByIDがnil
// として報告する) はフォーム全体のメッセージ1件として返します。このnilのケースと
// UseCase側のコード不一致のケースを意図的に同じメッセージにまとめ、idが実在の確認を
// 指していたかをフォームが漏らさないようにします。コード自体はここでは照合しません。
// 不一致は書き込み (試行回数のインクリメント) を要するため、照合はUseCaseの
// トランザクションに属します。本物のクエリ失敗は素のerrorとして表れます (上流で500
// として扱う)。
func (v *EmailConfirmationCreateValidator) Validate(ctx context.Context, input EmailConfirmationCreateValidatorInput) (*model.EmailConfirmation, error) {
	ve := model.NewValidationError()

	if input.Code == "" {
		ve.AddField("code", i18n.T(ctx, "validation_required"))
		return nil, ve
	}

	if !confirmationCodeRegex.MatchString(input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_code_invalid_format"))
		return nil, ve
	}

	confirmation, err := v.emailConfirmationRepo.FindActiveByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if confirmation == nil {
		ve.AddGlobal(i18n.T(ctx, "validation_code_incorrect_or_expired"))
		return nil, ve
	}

	return confirmation, nil
}
