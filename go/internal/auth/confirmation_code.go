package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// confirmationCodeMaxは確認コードの排他的上限です。コードは0から999999まで
// で、6桁ゼロ埋めの整形により000000-999999になります。
const confirmationCodeMax = 1000000

// GenerateConfirmationCodeは暗号論的乱数による6桁の数字確認コードを返します。
// 常に6桁で表示されるようゼロ埋めします。これはメールアドレスの管理権を検証する
// ため (例: サインアップ時) にメール送信され、ユーザーが入力し返すコードです。乱数
// プリミティブを1箇所に集約するため、呼び出し側のUseCaseではなくセキュアランダム
// ユーティリティであるauthに置きます。
func GenerateConfirmationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(confirmationCodeMax))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
