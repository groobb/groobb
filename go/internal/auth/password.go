// authパッケージはパスワードハッシュ化などの認証プリミティブを提供します。
//
// 標準ライブラリと外部の暗号ライブラリのみに依存し、Groobbの他パッケージには
// 依存しない純粋な技術ユーティリティです。内部依存を持たないことで、どの層からも
// 循環importのリスクなく呼び出せます。
package auth

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCostはパスワードハッシュ化に使うコストです。既定は
// bcrypt.DefaultCost (10) で、テストではハッシュ化を高速化するためTestBcryptCost
// に下げます。
var BcryptCost = bcrypt.DefaultCost

// TestBcryptCostはテストでハッシュ化を高速化するための最小bcryptコストです。
const TestBcryptCost = bcrypt.MinCost

// HashPasswordは与えられた平文パスワードをBcryptCostでbcryptハッシュ化
// します。bcryptはハッシュごとのソルトを生成して埋め込むため、同じパスワードでも
// 毎回異なるハッシュになります。
func HashPassword(plainPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPasswordはbcryptハッシュ化されたパスワードが平文パスワードと一致する
// かを返します。一致すればnilを、しなければ非nilのエラーを返します。
func CheckPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

const (
	// MinPasswordLengthは最小パスワード長で、エンコーディングによらず「8文字」
	// と読めるようrune単位で数えます (日本語パスワードはバイトではなく文字で数える)。
	MinPasswordLength = 8

	// MaxPasswordLengthはバイト単位の最大パスワード長です。bcryptは入力の
	// 先頭72バイトのみをハッシュ化し残りを黙って無視するため、72バイトで打ち切る
	// ことで、72バイトの接頭辞を共有する2つのパスワードが同一扱いされるのを防ぎます。
	MaxPasswordLength = 72
)

// ErrPasswordTooShort / ErrPasswordTooLongはValidatePasswordStrengthが
// 返すsentinel errorです。authをi18n非依存 (純粋な技術ユーティリティ) に保つため
// 翻訳済みメッセージではなくsentinelとし、呼び出し側 (validator) がerrors.Isで
// ローカライズ済みメッセージに対応づけます。
var (
	ErrPasswordTooShort = errors.New("password is too short")
	ErrPasswordTooLong  = errors.New("password is too long")
)

// ValidatePasswordStrengthはパスワードが長さポリシーを満たすか検証し、満たさ
// なければsentinel errorを返します。文字種は制限しません。非ASCIIパスワード
// (例: 日本語) を許可するため、最小はrune単位で、最大はbcryptの72バイト入力
// 制限を尊重してバイト単位で測ります。空パスワードはtoo shortとして失敗します。
// 「入力してください」を別途出したい呼び出し側は、本関数を呼ぶ前に空かどうかを
// 確認します。
func ValidatePasswordStrength(plainPassword string) error {
	if utf8.RuneCountInString(plainPassword) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(plainPassword) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}
