package validator

import (
	"context"
	"errors"
	"regexp"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// AtnameMaxLength is the maximum length of an atname, in characters. atname is
// restricted to ASCII (see atnameRegex), so the byte length equals the character
// count for any value that passes the format check. The form's maxlength mirrors
// this constant.
//
// [Ja] AtnameMaxLength は atname の最大文字数。atname は ASCII に限定される
// (atnameRegex 参照) ため、形式チェックを通る値ではバイト長と文字数が一致する。フォームの
// maxlength はこの定数をミラーする。
const AtnameMaxLength = 20

// atnameRegex is the allowed atname format: one or more ASCII letters, digits,
// or underscores. It requires at least one character, so an empty atname fails
// it; the empty case is reported as "required" before the format check runs so
// the two errors do not stack. The form's pattern mirrors this expression.
//
// [Ja] atnameRegex は許可する atname の形式: 1 文字以上の ASCII 英数字またはアンダー
// スコア。1 文字以上を要求するため空の atname は不適合になるが、空のケースは形式チェックの
// 前に「必須」として報告し 2 つのエラーが重ならないようにする。フォームの pattern はこの
// 式をミラーする。
var atnameRegex = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// IsValidAtname reports whether atname is one an account can hold: within
// AtnameMaxLength and matching the allowed format.
//
// AccountCreateValidator keeps the length and the format apart so that a form
// can report them as separate messages. A caller outside a form only needs to
// know whether the value is usable, and asking here rather than repeating the
// rule is what keeps a second copy of it from drifting away from this one.
//
// [Ja] IsValidAtname は、atname がアカウントの持てるもの (AtnameMaxLength 以内で、
// 許可された形式に適合するもの) かどうかを返します。
//
// AccountCreateValidator が長さと形式を分けているのは、フォームがそれらを別々の
// メッセージとして報告できるようにするためです。フォームの外の呼び出し元が知る必要が
// あるのは値が使えるかどうかだけであり、規則を書き写すのではなくここへ尋ねることが、
// 2 つ目の写しがこの規則から離れていくのを防ぎます。
func IsValidAtname(atname string) bool {
	return len(atname) <= AtnameMaxLength && atnameRegex.MatchString(atname)
}

// AccountCreateValidator validates the account-creation form: the chosen atname
// is well-formed and not already taken, the password meets the strength policy,
// and the confirmation field matches it. It depends on userRepo for the atname
// uniqueness check (a state check against the database); the email is not
// validated here since it comes from a verified confirmation, not user input.
//
// [Ja] AccountCreateValidator はアカウント作成フォームを検証します。選んだ atname の
// 形式が正しく既に使われておらず、パスワードが強度ポリシーを満たし、確認フィールドが
// 一致することです。atname の一意性チェック (DB に対する状態チェック) のため userRepo に
// 依存します。email はユーザー入力ではなく検証済みの確認から来るため、ここでは検証しません。
type AccountCreateValidator struct {
	userRepo *repository.UserRepository
}

// NewAccountCreateValidator creates an AccountCreateValidator.
//
// [Ja] NewAccountCreateValidator は AccountCreateValidator を生成します。
func NewAccountCreateValidator(userRepo *repository.UserRepository) *AccountCreateValidator {
	return &AccountCreateValidator{userRepo: userRepo}
}

// AccountCreateValidatorInput is the input to AccountCreateValidator.Validate.
//
// [Ja] AccountCreateValidatorInput は AccountCreateValidator.Validate の入力です。
type AccountCreateValidatorInput struct {
	Atname               string
	Password             string
	PasswordConfirmation string
}

// Validate checks the atname, password, and its confirmation, returning a
// *model.ValidationError when any is invalid, or a plain error on a genuine
// system failure (e.g. the database is unreachable). Format checks (atname shape
// and password strength) run first; only when they all pass is the atname
// uniqueness check made against the database, so a malformed atname never hits
// the database. An empty atname reports a required error and skips the format
// checks so the two errors do not stack. The match is case-insensitive because
// users.atname collates NOCASE.
//
// [Ja] Validate は atname・パスワード・その確認を検証し、いずれかが不正なら
// *model.ValidationError を、本物のシステム障害 (例: データベースに到達できない) では
// 素の error を返します。形式チェック (atname の形と パスワード強度) を先に行い、それらが
// すべて通ったときだけ atname の一意性チェックを DB に対して行うため、不正な atname が
// DB に到達することはありません。空の atname は必須エラーを報告し形式チェックをスキップ
// して 2 つのエラーが重ならないようにします。照合は users.atname が NOCASE 照合のため大文字
// 小文字を区別しません。
func (v *AccountCreateValidator) Validate(ctx context.Context, input AccountCreateValidatorInput) error {
	ve := model.NewValidationError()

	if input.Atname == "" {
		ve.AddField("atname", i18n.T(ctx, "validation_required"))
	} else {
		if len(input.Atname) > AtnameMaxLength {
			ve.AddField("atname", i18n.T(ctx, "validation_atname_too_long"))
		}
		if !atnameRegex.MatchString(input.Atname) {
			ve.AddField("atname", i18n.T(ctx, "validation_atname_invalid_format"))
		}
	}

	if input.Password == "" {
		ve.AddField("password", i18n.T(ctx, "validation_required"))
	} else {
		switch err := auth.ValidatePasswordStrength(input.Password); {
		case errors.Is(err, auth.ErrPasswordTooShort):
			ve.AddField("password", i18n.T(ctx, "validation_password_too_short"))
		case errors.Is(err, auth.ErrPasswordTooLong):
			ve.AddField("password", i18n.T(ctx, "validation_password_too_long"))
		}
	}

	// Check the confirmation only against a non-empty password: an empty password
	// already reported "required", and flagging a mismatch on top would be noise.
	//
	// [Ja] 確認は空でないパスワードに対してのみ照合する。空パスワードは既に「必須」を
	// 報告済みで、その上に不一致まで出すのはノイズになる。
	if input.PasswordConfirmation == "" {
		ve.AddField("password_confirmation", i18n.T(ctx, "validation_required"))
	} else if input.Password != "" && input.Password != input.PasswordConfirmation {
		ve.AddField("password_confirmation", i18n.T(ctx, "validation_password_mismatch"))
	}

	if ve.HasErrors() {
		return ve
	}

	// State check: the atname has passed the format checks, so it is safe to look
	// it up. A duplicate is reported explicitly; the DB UNIQUE constraint remains
	// the last line of defense for the rare check-then-insert race.
	//
	// [Ja] 状態チェック: atname は形式チェックを通過済みのため引いてよい。重複は明示的に
	// 報告する。稀な「チェックしてから挿入」の競合に対しては DB の UNIQUE 制約が最終防衛線
	// として残る。
	existingUser, err := v.userRepo.FindByAtname(ctx, input.Atname)
	if err != nil {
		return err
	}
	if existingUser != nil {
		ve.AddField("atname", i18n.T(ctx, "validation_atname_already_taken"))
		return ve
	}

	return nil
}
