package validator

import (
	"context"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SettingsWithdrawalDeleteValidator validates the account-withdrawal form: that
// the submitted current password matches the account's stored credential.
// Withdrawal is irreversible, so re-authenticating with the current password
// gates it (mirroring the email-change form), guarding against a left-open device
// or a stolen session triggering it. It needs only the password repository since
// the digest lives in user_passwords, not on users.
//
// [Ja] SettingsWithdrawalDeleteValidator は退会フォームを検証します。送信された現在の
// パスワードがアカウントの保存済み資格情報と一致することです。退会は不可逆なため、現在の
// パスワードでの再認証をゲートにし (メールアドレス変更フォームと同様)、放置端末や
// セッション盗用による発動を防ぎます。ダイジェストは users ではなく user_passwords に
// あるため、必要なのはパスワードリポジトリだけです。
type SettingsWithdrawalDeleteValidator struct {
	userPasswordRepo *repository.UserPasswordRepository
}

// NewSettingsWithdrawalDeleteValidator creates a
// SettingsWithdrawalDeleteValidator.
//
// [Ja] NewSettingsWithdrawalDeleteValidator は
// SettingsWithdrawalDeleteValidator を生成します。
func NewSettingsWithdrawalDeleteValidator(
	userPasswordRepo *repository.UserPasswordRepository,
) *SettingsWithdrawalDeleteValidator {
	return &SettingsWithdrawalDeleteValidator{
		userPasswordRepo: userPasswordRepo,
	}
}

// SettingsWithdrawalDeleteValidatorInput is the input to
// SettingsWithdrawalDeleteValidator.Validate. UserID identifies the signed-in user
// requesting withdrawal (established by the session, not the form), so the
// validator reads the password credential itself rather than trusting values from
// the client.
//
// [Ja] SettingsWithdrawalDeleteValidatorInput は
// SettingsWithdrawalDeleteValidator.Validate の入力です。UserID は退会を申請する
// サインイン済みユーザーを指し (フォームではなくセッションで確定する)、バリデーターは
// クライアント由来の値を信じずにパスワード資格情報を自身で読みます。
type SettingsWithdrawalDeleteValidatorInput struct {
	UserID          model.UserID
	CurrentPassword string
}

// Validate checks the submitted current password, returning a
// *model.ValidationError on any input problem or a plain error on a genuine system
// failure (e.g. the database is unreachable). The required check (current password
// present) runs first; only when it passes is the password matched against the
// stored credential, so an empty submission never hits the database.
//
// [Ja] Validate は送信された現在のパスワードを検証し、入力に問題があれば
// *model.ValidationError を、本物のシステム障害 (例: データベースに到達できない) では
// 素の error を返します。必須チェック (現在のパスワードの入力) を先に行い、それが通った
// ときだけ保存済み資格情報と照合するため、空の送信が DB に到達することはありません。
func (v *SettingsWithdrawalDeleteValidator) Validate(ctx context.Context, input SettingsWithdrawalDeleteValidatorInput) error {
	ve := model.NewValidationError()

	if input.CurrentPassword == "" {
		ve.AddField("current_password", i18n.T(ctx, "validation_required"))
		return ve
	}

	if err := v.validateCurrentPassword(ctx, ve, input.UserID, input.CurrentPassword); err != nil {
		return err
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

// validateCurrentPassword records a field error when the submitted current
// password does not match the account's stored credential. An account with no
// password credential (e.g. an SSO-only user) cannot prove the current password,
// so it is reported the same way as a wrong password rather than being singled
// out. A returned error is a genuine system failure; the field error is collected
// into ve.
//
// [Ja] validateCurrentPassword は送信された現在のパスワードがアカウントの保存済み資格
// 情報と一致しないときにフィールドエラーを記録します。パスワード資格情報の無い
// アカウント (例: SSO のみのユーザー) は現在のパスワードを証明できないため、特別扱いせず
// 誤ったパスワードと同じように報告します。返す error は本物のシステム障害で、フィールド
// エラーは ve に集約します。
func (v *SettingsWithdrawalDeleteValidator) validateCurrentPassword(ctx context.Context, ve *model.ValidationError, userID model.UserID, currentPassword string) error {
	password, err := v.userPasswordRepo.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if password == nil || auth.CheckPassword(password.PasswordDigest, currentPassword) != nil {
		ve.AddField("current_password", i18n.T(ctx, "validation_current_password_incorrect"))
	}
	return nil
}
