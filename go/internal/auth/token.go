package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// secureTokenBytesはセッショントークンの背後にあるランダムバイト数です。
// 24バイト (192ビット) はbase64urlエンコードでパディング無しの固定32文字に
// なり、プロジェクト間の一貫性のため姉妹Korylusプロジェクトに揃えています。
const secureTokenBytes = 24

// GenerateSecureTokenは不透明なセッショントークンとして使う、暗号論的乱数の
// URLセーフなトークンを返します。Cookie値として安全に格納できるようbase64url
// エンコードし、乱数プリミティブを1箇所に集約するためセキュアランダムユーティリティ
// であるauthに置きます。
func GenerateSecureToken() (string, error) {
	b := make([]byte, secureTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashTokenはトークンの16進エンコードしたSHA-256ダイジェストを返します。
// トークンを平文ではなくハッシュで保存・照合するために使います (例: DBが漏えいしても
// 使えるトークンが露出しないよう、ダイジェストとして保持するパスワードリセットトークン)。
//
// ここでSHA-256を使うのは意図的で、bcryptではありません。トークンは
// GenerateSecureTokenによる高エントロピーのランダム値 (低エントロピーの人間の秘密では
// ない) のため、ソルトや低速ハッシュは不要で、保存値を完全一致のルックアップで照合できる
// 高速で決定的なダイジェストこそが必要です。ダイジェストは暗号ユーティリティであるauthに
// 置き、ハッシュのプリミティブを1箇所に集約します。
func HashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
