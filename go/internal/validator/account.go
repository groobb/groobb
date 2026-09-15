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

// AtnameMaxLengthはatnameの最大文字数。atnameはASCIIに限定される
// (atnameRegex参照) ため、形式チェックを通る値ではバイト長と文字数が一致する。フォームの
// maxlengthはこの定数をミラーする。
const AtnameMaxLength = 20

// atnameRegexは許可するatnameの形式: 1文字以上のASCII英数字またはアンダー
// スコア。1文字以上を要求するため空のatnameは不適合になるが、空のケースは形式チェックの
// 前に「必須」として報告し2つのエラーが重ならないようにする。フォームのpatternはこの
// 式をミラーする。
var atnameRegex = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// IsValidAtnameは、atnameがアカウントの持てるもの (AtnameMaxLength以内で、
// 許可された形式に適合するもの) かどうかを返します。
//
// AccountCreateValidatorが長さと形式を分けているのは、フォームがそれらを別々の
// メッセージとして報告できるようにするためです。フォームの外の呼び出し元が知る必要が
// あるのは値が使えるかどうかだけであり、規則を書き写すのではなくここへ尋ねることが、
// 2つ目の写しがこの規則から離れていくのを防ぎます。
func IsValidAtname(atname string) bool {
	return len(atname) <= AtnameMaxLength && atnameRegex.MatchString(atname)
}

// AccountCreateValidatorはアカウント作成フォームを検証します。選んだatnameの
// 形式が正しく既に使われておらず、パスワードが強度ポリシーを満たし、確認フィールドが
// 一致することです。atnameの一意性チェック (DBに対する状態チェック) のためuserRepoに
// 依存します。emailはユーザー入力ではなく検証済みの確認から来るため、ここでは検証しません。
type AccountCreateValidator struct {
	userRepo *repository.UserRepository
}

// NewAccountCreateValidatorはAccountCreateValidatorを生成します。
func NewAccountCreateValidator(userRepo *repository.UserRepository) *AccountCreateValidator {
	return &AccountCreateValidator{userRepo: userRepo}
}

// AccountCreateValidatorInputはAccountCreateValidator.Validateの入力です。
type AccountCreateValidatorInput struct {
	Atname               string
	Password             string
	PasswordConfirmation string
}

// Validateはatname・パスワード・その確認を検証し、いずれかが不正なら
// *model.ValidationErrorを、本物のシステム障害 (例: データベースに到達できない) では
// 素のerrorを返します。形式チェック (atnameの形と パスワード強度) を先に行い、それらが
// すべて通ったときだけatnameの一意性チェックをDBに対して行うため、不正なatnameが
// DBに到達することはありません。空のatnameは必須エラーを報告し形式チェックをスキップ
// して2つのエラーが重ならないようにします。照合はusers.atnameがNOCASE照合のため大文字
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

	// 確認は空でないパスワードに対してのみ照合する。空パスワードは既に「必須」を
	// 報告済みで、その上に不一致まで出すのはノイズになる。
	if input.PasswordConfirmation == "" {
		ve.AddField("password_confirmation", i18n.T(ctx, "validation_required"))
	} else if input.Password != "" && input.Password != input.PasswordConfirmation {
		ve.AddField("password_confirmation", i18n.T(ctx, "validation_password_mismatch"))
	}

	if ve.HasErrors() {
		return ve
	}

	// 状態チェック: atnameは形式チェックを通過済みのため引いてよい。重複は明示的に
	// 報告する。稀な「チェックしてから挿入」の競合に対してはDBのUNIQUE制約が最終防衛線
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
