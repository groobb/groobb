package validator

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SettingsEmailUpdateValidatorはメールアドレス変更申請フォームを検証します。
// 新しいアドレスが入力され形式が正しく、アカウントの現在のアドレスと異なり、既に
// 使われておらず、送信された現在のパスワードが一致することです。アカウントの現在の
// emailを読み新しいアドレスの重複を調べるためのユーザーリポジトリと、現在のパスワードの
// 照合対象となる資格情報を取るためのパスワードリポジトリを必要とします (ダイジェストは
// usersではなくuser_passwordsにあるためです)。
type SettingsEmailUpdateValidator struct {
	userRepo         *repository.UserRepository
	userPasswordRepo *repository.UserPasswordRepository
}

// NewSettingsEmailUpdateValidatorはSettingsEmailUpdateValidatorを生成します。
func NewSettingsEmailUpdateValidator(
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *SettingsEmailUpdateValidator {
	return &SettingsEmailUpdateValidator{
		userRepo:         userRepo,
		userPasswordRepo: userPasswordRepo,
	}
}

// SettingsEmailUpdateValidatorInputはSettingsEmailUpdateValidator.Validate
// の入力です。UserIDは変更を申請するサインイン済みユーザーを指し (フォームではなく
// セッションで確定する)、バリデーターはクライアントがエコーバックした値を信じずに
// 現在のemailとパスワードを自身で読みます。
type SettingsEmailUpdateValidatorInput struct {
	UserID          model.UserID
	NewEmail        string
	CurrentPassword string
}

// Validateは送信された新しいemailと現在のパスワードを検証し、入力に問題が
// あれば *model.ValidationErrorを、本物のシステム障害 (例: データベースに到達できない)
// では素のerrorを返します。形式チェック (新しいemailの必須・形式、現在のパスワードの
// 必須) を先に行い、それらが通ったときだけ状態チェックをDBに対して行うため、不正な
// リクエストがDBに到達することはありません。emailの照合はusers.emailがNOCASE照合のため、
// 未変更チェックと重複チェックのどちらも大文字小文字を区別しません。
func (v *SettingsEmailUpdateValidator) Validate(ctx context.Context, input SettingsEmailUpdateValidatorInput) error {
	ve := model.NewValidationError()

	if input.NewEmail == "" {
		ve.AddField("email", i18n.T(ctx, "validation_required"))
	} else if _, err := mail.ParseAddress(input.NewEmail); err != nil {
		ve.AddField("email", i18n.T(ctx, "validation_email_invalid"))
	}

	if input.CurrentPassword == "" {
		ve.AddField("current_password", i18n.T(ctx, "validation_required"))
	}

	if ve.HasErrors() {
		return ve
	}

	if err := v.validateNewEmail(ctx, ve, input.UserID, input.NewEmail); err != nil {
		return err
	}
	if err := v.validateCurrentPassword(ctx, ve, input.UserID, input.CurrentPassword); err != nil {
		return err
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

// validateNewEmailは新しいアドレスが現在のものと同じ (変更対象が無い) か、既に
// 別アカウントに使われているときにフィールドエラーを記録します。申請ユーザーから現在の
// emailを読み、重複ルックアップの前に未変更のアドレスを弾いて「現在と同じ」と「既に
// 使用済み」が重ならないようにします。重複チェックは見つかったアカウントのidを申請者の
// ものと比較し、未変更チェックをすり抜けた大小差が「使用済み」と誤報されない
// ようにします。稀な「チェックしてから更新」の競合にはusers.emailのUNIQUE制約が最終
// 防衛線として残ります。返すerrorは本物のシステム障害で、フィールドエラーはveに
// 集約します。
func (v *SettingsEmailUpdateValidator) validateNewEmail(ctx context.Context, ve *model.ValidationError, userID model.UserID, newEmail string) error {
	currentUser, err := v.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	// ユーザーは認証済みセッションから解決されるため存在するはず。ここでのnilは
	// ユーザーが修正できる入力エラーではなく想定外の状態である。
	if currentUser == nil {
		return fmt.Errorf("メールアドレス変更申請者のユーザーが見つからない: id=%s", userID)
	}

	if strings.EqualFold(newEmail, currentUser.Email) {
		ve.AddField("email", i18n.T(ctx, "validation_email_unchanged"))
		return nil
	}

	existingUser, err := v.userRepo.FindByEmail(ctx, newEmail)
	if err != nil {
		return err
	}
	if existingUser != nil && existingUser.ID != userID {
		ve.AddField("email", i18n.T(ctx, "validation_email_already_taken"))
	}
	return nil
}

// validateCurrentPasswordは送信された現在のパスワードがアカウントの保存済み資格
// 情報と一致しないときにフィールドエラーを記録します。パスワード資格情報の無い
// アカウント (例: SSOのみのユーザー) は現在のパスワードを証明できないため、特別扱いせず
// 誤ったパスワードと同じように報告します。返すerrorは本物のシステム障害で、フィールド
// エラーはveに集約します。
func (v *SettingsEmailUpdateValidator) validateCurrentPassword(ctx context.Context, ve *model.ValidationError, userID model.UserID, currentPassword string) error {
	password, err := v.userPasswordRepo.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if password == nil || auth.CheckPassword(password.PasswordDigest, currentPassword) != nil {
		ve.AddField("current_password", i18n.T(ctx, "validation_current_password_incorrect"))
	}
	return nil
}
