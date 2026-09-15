package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/groobb/groobb/go/internal/config"
)

type continuationTokenPurpose string

const (
	continuationTokenVersion      = "v1"
	emailConfirmationTokenPurpose = continuationTokenPurpose("email_confirmation")
	twoFactorPendingTokenPurpose  = continuationTokenPurpose("two_factor_pending")
)

// signContinuationTokenは正のデータベースid・認証フローの用途・サーバー側の
// 有効期限をHMAC-SHA-256署名へ結び付けます。tokenはCookie値として安全でid自体は
// 見えますが、設定済みの鍵なしに改ざん・新規発行はできません。鍵またはidが不正なら空の
// tokenとしてfail-closedにし、実行中アプリではConfig.Loadがその状態を防ぎます。
func signContinuationToken(key string, purpose continuationTokenPurpose, id int64, expiresAt time.Time) string {
	if len(key) < config.ContinuationTokenMinimumKeyLength || id <= 0 {
		return ""
	}

	payload := strings.Join([]string{
		continuationTokenVersion,
		string(purpose),
		strconv.FormatInt(id, 10),
		strconv.FormatInt(expiresAt.Unix(), 10),
	}, ".")
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return payload + "." + signature
}

// verifyContinuationTokenはtokenを認証してからidを返します。形式不正、別フローの
// 用途、期限切れ、正でないid、設定済みの鍵で生成されていない署名をすべて拒否します。
func verifyContinuationToken(key string, expectedPurpose continuationTokenPurpose, token string, now time.Time) (int64, bool) {
	if len(key) < config.ContinuationTokenMinimumKeyLength {
		return 0, false
	}

	parts := strings.Split(token, ".")
	if len(parts) != 5 || parts[0] != continuationTokenVersion || parts[1] != string(expectedPurpose) {
		return 0, false
	}

	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, false
	}
	payload := strings.Join(parts[:4], ".")
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(payload))
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		return 0, false
	}

	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || now.Unix() >= expiresAt {
		return 0, false
	}

	return id, true
}
