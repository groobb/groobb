package validator

import (
	"context"
	"regexp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// confirmationCodeRegex matches a six-digit numeric confirmation code, the form
// GenerateConfirmationCode emits. It is compiled once at package level so the
// check costs nothing per request.
//
// [Ja] confirmationCodeRegex は 6 桁の数字確認コード (GenerateConfirmationCode が
// 発行する形式) にマッチします。リクエストごとの検証コストをゼロにするため、パッケージ
// レベルで 1 度だけコンパイルします。
var confirmationCodeRegex = regexp.MustCompile(`^\d{6}$`)

// EmailConfirmationCreateValidator validates a submitted confirmation code's
// format and resolves the still-active confirmation for the id carried from
// sign-up. It does not compare the code against the stored value: a mismatch
// must increment the confirmation's failed-attempt count (a DB write), so per
// the validation guideline the code comparison and that increment live in the
// UseCase's transaction, not here. The checks that remain (format, and whether
// an active confirmation exists) never write, so they belong in the validator.
//
// [Ja] EmailConfirmationCreateValidator は送信された確認コードの形式を検証し、
// サインアップから運ばれた id のまだ有効な確認を解決します。コードを保存値と照合は
// しません。不一致は確認の失敗試行回数をインクリメント (DB 書き込み) する必要があり、
// バリデーションガイドラインに従いコード照合とそのインクリメントは UseCase の
// トランザクションに置くためです。ここに残る検証 (形式、およびアクティブな確認が
// 存在するか) はいずれも書き込まないため validator に属します。
type EmailConfirmationCreateValidator struct {
	emailConfirmationRepo *repository.EmailConfirmationRepository
}

// NewEmailConfirmationCreateValidator creates an EmailConfirmationCreateValidator.
//
// [Ja] NewEmailConfirmationCreateValidator は EmailConfirmationCreateValidator を
// 生成します。
func NewEmailConfirmationCreateValidator(emailConfirmationRepo *repository.EmailConfirmationRepository) *EmailConfirmationCreateValidator {
	return &EmailConfirmationCreateValidator{emailConfirmationRepo: emailConfirmationRepo}
}

// EmailConfirmationCreateValidatorInput is the input to Validate. ID is the
// pending confirmation's id (read from the handoff cookie by the handler), and
// Code is the value the user typed.
//
// [Ja] EmailConfirmationCreateValidatorInput は Validate の入力です。ID は保留中の
// 確認の id (ハンドラーが受け渡し Cookie から読む)、Code はユーザーが入力した値です。
type EmailConfirmationCreateValidatorInput struct {
	ID   model.EmailConfirmationID
	Code string
}

// Validate checks the code's format and returns the still-active confirmation
// for the id so the UseCase can compare the code and stamp or count the attempt
// without re-querying. Format problems attach to the code field; a missing,
// expired, or attempt-exhausted confirmation (all of which FindActiveByID
// reports as nil) is returned as a single form-wide message. That nil case and
// the UseCase's wrong-code case deliberately share the same message so the form
// never reveals whether the id pointed at a real confirmation. The code itself
// is not compared here — a mismatch must write (increment the attempt count), so
// the comparison belongs in the UseCase's transaction. A genuine query failure
// surfaces as a plain error (handled as 500 upstream).
//
// [Ja] Validate はコードの形式を検証し、id のまだ有効な確認を返して、UseCase が再クエリ
// せずにコードを照合し打刻または試行回数の計上をできるようにします。形式の問題は code
// フィールドに付け、確認が無い / 期限切れ / 試行回数超過 (いずれも FindActiveByID が nil
// として報告する) はフォーム全体のメッセージ 1 件として返します。この nil のケースと
// UseCase 側のコード不一致のケースを意図的に同じメッセージにまとめ、id が実在の確認を
// 指していたかをフォームが漏らさないようにします。コード自体はここでは照合しません。
// 不一致は書き込み (試行回数のインクリメント) を要するため、照合は UseCase の
// トランザクションに属します。本物のクエリ失敗は素の error として表れます (上流で 500
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
