package auth

import (
	"crypto/rand"
	"math/big"
)

const (
	// RecoveryCodeCount is how many recovery codes are issued when 2FA is enabled.
	//
	// [Ja] RecoveryCodeCount は 2FA 有効化時に発行するリカバリーコードの個数です。
	RecoveryCodeCount = 10

	// RecoveryCodeLength is the character length of each recovery code.
	//
	// [Ja] RecoveryCodeLength は各リカバリーコードの文字数です。
	RecoveryCodeLength = 8

	// recoveryCodeAlphabet is the set of characters a recovery code is drawn from:
	// lowercase letters and digits, chosen so codes are easy to read and type.
	//
	// [Ja] recoveryCodeAlphabet はリカバリーコードを構成する文字集合です。読みやすく
	// 入力しやすいよう、小文字と数字から選びます。
	recoveryCodeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// GenerateRecoveryCodes returns RecoveryCodeCount fresh recovery codes, each a
// RecoveryCodeLength-character lowercase-alphanumeric string. They are the
// one-time backup codes shown once when 2FA is enabled and used to sign in when
// the authenticator app is unavailable; a used code is later removed from the
// stored set. They live in auth (the secure-random utility) so the randomness
// primitive stays in one place.
//
// [Ja] GenerateRecoveryCodes は RecoveryCodeCount 個の新しいリカバリーコードを返します。
// 各コードは RecoveryCodeLength 文字の小文字英数字です。これらは 2FA 有効化時に一度だけ
// 表示され、認証アプリが使えないときにサインインするための 1 回使い切りのバックアップ
// コードで、使用したコードは後で保存済みの集合から削除されます。乱数プリミティブを 1 箇所に
// 集約するため、セキュアランダムユーティリティである auth に置きます。
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, RecoveryCodeCount)
	for i := range codes {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

// generateRecoveryCode returns a single cryptographically random recovery code of
// RecoveryCodeLength characters drawn uniformly from recoveryCodeAlphabet.
//
// [Ja] generateRecoveryCode は recoveryCodeAlphabet から一様に選んだ RecoveryCodeLength
// 文字から成る、暗号論的乱数による 1 つのリカバリーコードを返します。
func generateRecoveryCode() (string, error) {
	b := make([]byte, RecoveryCodeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(recoveryCodeAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = recoveryCodeAlphabet[n.Int64()]
	}
	return string(b), nil
}
