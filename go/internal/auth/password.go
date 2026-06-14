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

import "golang.org/x/crypto/bcrypt"

// BcryptCost is the cost used for password hashing. It defaults to
// bcrypt.DefaultCost (10); tests lower it to TestBcryptCost to speed up hashing.
//
// [Ja] BcryptCost はパスワードハッシュ化に使うコストです。既定は
// bcrypt.DefaultCost (10) で、テストではハッシュ化を高速化するため TestBcryptCost
// に下げます。
var BcryptCost = bcrypt.DefaultCost

// TestBcryptCost is the minimal bcrypt cost used in tests to speed up hashing.
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
