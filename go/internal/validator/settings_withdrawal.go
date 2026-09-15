package validator

import (
	"context"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SettingsWithdrawalDeleteValidatorは退会フォームを検証します。送信された現在の
// パスワードがアカウントの保存済み資格情報と一致することです。退会は不可逆なため、現在の
// パスワードでの再認証をゲートにし (メールアドレス変更フォームと同様)、放置端末や
// セッション盗用による発動を防ぎます。ダイジェストはusersではなくuser_passwordsに
// あるため、必要なのはパスワードリポジトリだけです。
type SettingsWithdrawalDeleteValidator struct {
	userPasswordRepo *repository.UserPasswordRepository
}

// NewSettingsWithdrawalDeleteValidatorは
// SettingsWithdrawalDeleteValidatorを生成します。
func NewSettingsWithdrawalDeleteValidator(
	userPasswordRepo *repository.UserPasswordRepository,
) *SettingsWithdrawalDeleteValidator {
	return &SettingsWithdrawalDeleteValidator{
		userPasswordRepo: userPasswordRepo,
	}
}

// SettingsWithdrawalDeleteValidatorInputは
// SettingsWithdrawalDeleteValidator.Validateの入力です。UserIDは退会を申請する
// サインイン済みユーザーを指し (フォームではなくセッションで確定する)、バリデーターは
// クライアント由来の値を信じずにパスワード資格情報を自身で読みます。
type SettingsWithdrawalDeleteValidatorInput struct {
	UserID          model.UserID
	CurrentPassword string
}

// Validateは送信された現在のパスワードを検証し、入力に問題があれば
// *model.ValidationErrorを、本物のシステム障害 (例: データベースに到達できない) では
// 素のerrorを返します。必須チェック (現在のパスワードの入力) を先に行い、それが通った
// ときだけ保存済み資格情報と照合するため、空の送信がDBに到達することはありません。
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

// validateCurrentPasswordは送信された現在のパスワードがアカウントの保存済み資格
// 情報と一致しないときにフィールドエラーを記録します。パスワード資格情報の無い
// アカウント (例: SSOのみのユーザー) は現在のパスワードを証明できないため、特別扱いせず
// 誤ったパスワードと同じように報告します。返すerrorは本物のシステム障害で、フィールド
// エラーはveに集約します。
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
