// Package auth provides authentication primitives such as password hashing.
//
// It is a pure technical utility: it depends only on the standard library and
// external crypto libraries, never on other Groobb packages. Keeping it free of
// internal dependencies lets every layer call it without risking import cycles.
//
// [Ja] auth パッケージはパスワードハッシュ化などの認証プリミティブを提供します。
//
// 標準ライブラリと外部の暗号ライブラリのみに依存し、Groobb の他パッケージには
// 依存しない純粋な技術ユーティリティです。内部依存を持たないことで、どの層からも
// 循環 import のリスクなく呼び出せます。
package auth

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the cost used for password hashing. It defaults to
// bcrypt.DefaultCost (10); tests lower it to TestBcryptCost to speed up hashing.
//
// [Ja] BcryptCost はパスワードハッシュ化に使うコストです。既定は
// bcrypt.DefaultCost (10) で、テストではハッシュ化を高速化するため TestBcryptCost
// に下げます。
var BcryptCost = bcrypt.DefaultCost

// TestBcryptCost is the minimal bcrypt cost used in tests to speed up hashing.
//
// [Ja] TestBcryptCost はテストでハッシュ化を高速化するための最小 bcrypt コストです。
const TestBcryptCost = bcrypt.MinCost

// HashPassword hashes the given plaintext password with bcrypt using BcryptCost.
// bcrypt generates and embeds a per-hash salt, so the same password yields a
// different hash each time.
//
// [Ja] HashPassword は与えられた平文パスワードを BcryptCost で bcrypt ハッシュ化
// します。bcrypt はハッシュごとのソルトを生成して埋め込むため、同じパスワードでも
// 毎回異なるハッシュになります。
func HashPassword(plainPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether the bcrypt-hashed password matches the plaintext
// password, returning nil on a match and a non-nil error otherwise.
//
// [Ja] CheckPassword は bcrypt ハッシュ化されたパスワードが平文パスワードと一致する
// かを返します。一致すれば nil を、しなければ非 nil のエラーを返します。
func CheckPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

const (
	// MinPasswordLength is the minimum password length, measured in runes so the
	// limit reads as "8 characters" regardless of encoding (a Japanese password
	// counts by character, not byte).
	//
	// [Ja] MinPasswordLength は最小パスワード長で、エンコーディングによらず「8 文字」
	// と読めるよう rune 単位で数えます (日本語パスワードはバイトではなく文字で数える)。
	MinPasswordLength = 8

	// MaxPasswordLength is the maximum password length in bytes. bcrypt only
	// hashes the first 72 bytes of its input and silently ignores the rest, so
	// capping at 72 bytes prevents two passwords sharing a 72-byte prefix from
	// being treated as equal.
	//
	// [Ja] MaxPasswordLength はバイト単位の最大パスワード長です。bcrypt は入力の
	// 先頭 72 バイトのみをハッシュ化し残りを黙って無視するため、72 バイトで打ち切る
	// ことで、72 バイトの接頭辞を共有する 2 つのパスワードが同一扱いされるのを防ぎます。
	MaxPasswordLength = 72
)

// ErrPasswordTooShort and ErrPasswordTooLong are the sentinel errors
// ValidatePasswordStrength returns. They are sentinels rather than translated
// messages so auth stays free of i18n (a pure technical utility): the caller
// (the validator) maps them to localized messages with errors.Is.
//
// [Ja] ErrPasswordTooShort / ErrPasswordTooLong は ValidatePasswordStrength が
// 返す sentinel error です。auth を i18n 非依存 (純粋な技術ユーティリティ) に保つため
// 翻訳済みメッセージではなく sentinel とし、呼び出し側 (validator) が errors.Is で
// ローカライズ済みメッセージに対応づけます。
var (
	ErrPasswordTooShort = errors.New("password is too short")
	ErrPasswordTooLong  = errors.New("password is too long")
)

// ValidatePasswordStrength checks that a password meets the length policy,
// returning a sentinel error otherwise. It does not restrict the character set:
// non-ASCII passwords (e.g. Japanese) are allowed, so the minimum is measured in
// runes while the maximum is measured in bytes to respect bcrypt's 72-byte input
// limit. An empty password fails as too short; callers that want a distinct
// "required" message check for emptiness before calling this.
//
// [Ja] ValidatePasswordStrength はパスワードが長さポリシーを満たすか検証し、満たさ
// なければ sentinel error を返します。文字種は制限しません。非 ASCII パスワード
// (例: 日本語) を許可するため、最小は rune 単位で、最大は bcrypt の 72 バイト入力
// 制限を尊重してバイト単位で測ります。空パスワードは too short として失敗します。
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
