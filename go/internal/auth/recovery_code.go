package auth

import (
	"crypto/rand"
	"math/big"
)

const (
	// RecoveryCodeCountは2FA有効化時に発行するリカバリーコードの個数です。
	RecoveryCodeCount = 10

	// RecoveryCodeLengthは各リカバリーコードの文字数です。
	RecoveryCodeLength = 8

	// recoveryCodeAlphabetはリカバリーコードを構成する文字集合です。読みやすく
	// 入力しやすいよう、小文字と数字から選びます。
	recoveryCodeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// GenerateRecoveryCodesはRecoveryCodeCount個の新しいリカバリーコードを返します。
// 各コードはRecoveryCodeLength文字の小文字英数字です。これらは2FA有効化時に一度だけ
// 表示され、認証アプリが使えないときにサインインするための1回使い切りのバックアップ
// コードで、使用したコードは後で保存済みの集合から削除されます。乱数プリミティブを1箇所に
// 集約するため、セキュアランダムユーティリティであるauthに置きます。
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

// generateRecoveryCodeはrecoveryCodeAlphabetから一様に選んだRecoveryCodeLength
// 文字から成る、暗号論的乱数による1つのリカバリーコードを返します。
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
