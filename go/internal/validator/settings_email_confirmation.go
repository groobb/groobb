package validator

import (
	"context"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SettingsEmailConfirmationCreateValidator validates a submitted email-change
// confirmation code's format and resolves the still-active email-change
// confirmation for the signed-in user. It mirrors the sign-up
// EmailConfirmationCreateValidator but is keyed by the user id (taken from the
// session) rather than a confirmation id carried in a handoff cookie, because the
// email-change flow identifies the pending confirmation from the authenticated
// user. It does not compare the code against the stored value: a mismatch must
// increment the confirmation's failed-attempt count (a DB write), so per the
// validation guideline the code comparison and that increment live in the
// UseCase's transaction, not here. The checks that remain (format, and whether an
// active confirmation exists) never write, so they belong in the validator.
//
// [Ja] SettingsEmailConfirmationCreateValidator は送信されたメール変更の確認コードの
// 形式を検証し、サインイン済みユーザーのまだ有効なメール変更の確認を解決します。
// サインアップの EmailConfirmationCreateValidator と対をなしますが、受け渡し Cookie が
// 運ぶ確認 id ではなく (セッションから得る) ユーザー id をキーにします。メール変更フローは
// 認証済みユーザーから保留中の確認を特定するためです。コードを保存値と照合はしません。
// 不一致は確認の失敗試行回数をインクリメント (DB 書き込み) する必要があり、バリデーション
// ガイドラインに従いコード照合とそのインクリメントは UseCase のトランザクションに置くため
// です。ここに残る検証 (形式、およびアクティブな確認が存在するか) はいずれも書き込まない
// ため validator に属します。
type SettingsEmailConfirmationCreateValidator struct {
	emailConfirmationRepo *repository.EmailConfirmationRepository
}

// NewSettingsEmailConfirmationCreateValidator creates a
// SettingsEmailConfirmationCreateValidator.
//
// [Ja] NewSettingsEmailConfirmationCreateValidator は
// SettingsEmailConfirmationCreateValidator を生成します。
func NewSettingsEmailConfirmationCreateValidator(emailConfirmationRepo *repository.EmailConfirmationRepository) *SettingsEmailConfirmationCreateValidator {
	return &SettingsEmailConfirmationCreateValidator{emailConfirmationRepo: emailConfirmationRepo}
}

// SettingsEmailConfirmationCreateValidatorInput is the input to Validate. UserID
// is the signed-in user submitting the code (established by the session), and
// Code is the value the user typed.
//
// [Ja] SettingsEmailConfirmationCreateValidatorInput は Validate の入力です。UserID は
// コードを送信するサインイン済みユーザー (セッションで確定する)、Code はユーザーが
// 入力した値です。
type SettingsEmailConfirmationCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validate checks the code's format and returns the user's still-active
// email-change confirmation so the UseCase can compare the code and stamp or count
// the attempt without re-querying. Format problems attach to the code field; a
// missing, expired, or attempt-exhausted confirmation (all of which
// FindActiveEmailChangeByUserID reports as nil) is returned as a single form-wide
// message. That nil case and the UseCase's wrong-code case deliberately share the
// same message so the form never reveals whether a real confirmation was pending.
// The code itself is not compared here — a mismatch must write (increment the
// attempt count), so the comparison belongs in the UseCase's transaction. A
// genuine query failure surfaces as a plain error (handled as 500 upstream).
//
// [Ja] Validate はコードの形式を検証し、ユーザーのまだ有効なメール変更の確認を返して、
// UseCase が再クエリせずにコードを照合し打刻または試行回数の計上をできるようにします。
// 形式の問題は code フィールドに付け、確認が無い / 期限切れ / 試行回数超過 (いずれも
// FindActiveEmailChangeByUserID が nil として報告する) はフォーム全体のメッセージ 1 件と
// して返します。この nil のケースと UseCase 側のコード不一致のケースを意図的に同じ
// メッセージにまとめ、実在の確認が保留中だったかをフォームが漏らさないようにします。
// コード自体はここでは照合しません。不一致は書き込み (試行回数のインクリメント) を要する
// ため、照合は UseCase のトランザクションに属します。本物のクエリ失敗は素の error として
// 表れます (上流で 500 として扱う)。
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
