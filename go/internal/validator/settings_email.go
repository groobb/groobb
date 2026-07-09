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

// SettingsEmailUpdateValidator validates the email-change request form: that the
// new address is present, well-formed, different from the account's current
// address, and not already taken, and that the submitted current password
// matches. It needs the user repository to read the account's current email and
// to check the new address for a duplicate, and the password repository to fetch
// the credential the current password is checked against (the digest lives in
// user_passwords, not on users).
//
// [Ja] SettingsEmailUpdateValidator はメールアドレス変更申請フォームを検証します。
// 新しいアドレスが入力され形式が正しく、アカウントの現在のアドレスと異なり、既に
// 使われておらず、送信された現在のパスワードが一致することです。アカウントの現在の
// email を読み新しいアドレスの重複を調べるためのユーザーリポジトリと、現在のパスワードの
// 照合対象となる資格情報を取るためのパスワードリポジトリを必要とします (ダイジェストは
// users ではなく user_passwords にあるためです)。
type SettingsEmailUpdateValidator struct {
	userRepo         *repository.UserRepository
	userPasswordRepo *repository.UserPasswordRepository
}

// NewSettingsEmailUpdateValidator creates a SettingsEmailUpdateValidator.
//
// [Ja] NewSettingsEmailUpdateValidator は SettingsEmailUpdateValidator を生成します。
func NewSettingsEmailUpdateValidator(
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *SettingsEmailUpdateValidator {
	return &SettingsEmailUpdateValidator{
		userRepo:         userRepo,
		userPasswordRepo: userPasswordRepo,
	}
}

// SettingsEmailUpdateValidatorInput is the input to
// SettingsEmailUpdateValidator.Validate. UserID identifies the signed-in user
// requesting the change (established by the session, not the form), so the
// validator reads the current email and password itself rather than trusting
// values echoed by the client.
//
// [Ja] SettingsEmailUpdateValidatorInput は SettingsEmailUpdateValidator.Validate
// の入力です。UserID は変更を申請するサインイン済みユーザーを指し (フォームではなく
// セッションで確定する)、バリデーターはクライアントがエコーバックした値を信じずに
// 現在の email とパスワードを自身で読みます。
type SettingsEmailUpdateValidatorInput struct {
	UserID          model.UserID
	NewEmail        string
	CurrentPassword string
}

// Validate checks the submitted new email and current password, returning a
// *model.ValidationError on any input problem or a plain error on a genuine
// system failure (e.g. the database is unreachable). Format checks (new email
// required and well-formed, current password required) run first; only when they
// pass are the state checks made against the database, so a malformed request
// never hits it. The email match is case-insensitive because users.email is
// citext, both for the unchanged check and the duplicate check.
//
// [Ja] Validate は送信された新しい email と現在のパスワードを検証し、入力に問題が
// あれば *model.ValidationError を、本物のシステム障害 (例: データベースに到達できない)
// では素の error を返します。形式チェック (新しい email の必須・形式、現在のパスワードの
// 必須) を先に行い、それらが通ったときだけ状態チェックを DB に対して行うため、不正な
// リクエストが DB に到達することはありません。email の照合は users.email が citext のため、
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

// validateNewEmail records a field error when the new address equals the current
// one (nothing to change) or is already taken by another account. It reads the
// current email from the requesting user and rejects an unchanged address before
// the duplicate lookup, so "same as now" and "already taken" never stack. The
// duplicate check compares the found account's id against the requester's so a
// citext casing difference that slips past the unchanged check is not misreported
// as taken; the users.email UNIQUE constraint remains the last line of defense
// for the rare check-then-update race. A returned error is a genuine system
// failure; field errors are collected into ve.
//
// [Ja] validateNewEmail は新しいアドレスが現在のものと同じ (変更対象が無い) か、既に
// 別アカウントに使われているときにフィールドエラーを記録します。申請ユーザーから現在の
// email を読み、重複ルックアップの前に未変更のアドレスを弾いて「現在と同じ」と「既に
// 使用済み」が重ならないようにします。重複チェックは見つかったアカウントの id を申請者の
// ものと比較し、未変更チェックをすり抜けた citext の大小差が「使用済み」と誤報されない
// ようにします。稀な「チェックしてから更新」の競合には users.email の UNIQUE 制約が最終
// 防衛線として残ります。返す error は本物のシステム障害で、フィールドエラーは ve に
// 集約します。
func (v *SettingsEmailUpdateValidator) validateNewEmail(ctx context.Context, ve *model.ValidationError, userID model.UserID, newEmail string) error {
	currentUser, err := v.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	// The user is resolved from the authenticated session, so it must exist; a nil
	// here is an unexpected state rather than a user-fixable input error.
	//
	// [Ja] ユーザーは認証済みセッションから解決されるため存在するはず。ここでの nil は
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
