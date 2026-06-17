package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// secureTokenBytes is the number of random bytes behind a session token. 24
// bytes (192 bits) base64url-encode to a fixed 32-character string with no
// padding, matching the sister Korylus projects for cross-project consistency.
//
// [Ja] secureTokenBytes はセッショントークンの背後にあるランダムバイト数です。
// 24 バイト (192 ビット) は base64url エンコードでパディング無しの固定 32 文字に
// なり、プロジェクト間の一貫性のため姉妹 Korylus プロジェクトに揃えています。
const secureTokenBytes = 24

// GenerateSecureToken returns a cryptographically random, URL-safe token used as
// an opaque session token. It is base64url-encoded so it is safe to store in a
// cookie value, and lives in auth (the secure-random utility) so the randomness
// primitive stays in one place.
//
// [Ja] GenerateSecureToken は不透明なセッショントークンとして使う、暗号論的乱数の
// URL セーフなトークンを返します。Cookie 値として安全に格納できるよう base64url
// エンコードし、乱数プリミティブを 1 箇所に集約するためセキュアランダムユーティリティ
// である auth に置きます。
func GenerateSecureToken() (string, error) {
	b := make([]byte, secureTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashToken returns the hex-encoded SHA-256 digest of a token, used to store and
// look up a token by its hash rather than its plaintext (e.g. a password reset
// token kept as a digest so a database leak does not expose usable tokens).
//
// SHA-256 is deliberate here, not bcrypt: the token is a high-entropy random
// value from GenerateSecureToken (not a low-entropy human secret), so it needs
// no salting or slow hashing, and a fast deterministic digest is what lets the
// stored value be matched by an exact lookup. The digest lives in auth, the
// crypto utility, so the hashing primitive stays in one place.
//
// [Ja] HashToken はトークンの 16 進エンコードした SHA-256 ダイジェストを返します。
// トークンを平文ではなくハッシュで保存・照合するために使います (例: DB が漏えいしても
// 使えるトークンが露出しないよう、ダイジェストとして保持するパスワードリセットトークン)。
//
// ここで SHA-256 を使うのは意図的で、bcrypt ではありません。トークンは
// GenerateSecureToken による高エントロピーのランダム値 (低エントロピーの人間の秘密では
// ない) のため、ソルトや低速ハッシュは不要で、保存値を完全一致のルックアップで照合できる
// 高速で決定的なダイジェストこそが必要です。ダイジェストは暗号ユーティリティである auth に
// 置き、ハッシュのプリミティブを 1 箇所に集約します。
func HashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
