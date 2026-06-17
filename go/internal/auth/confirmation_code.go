package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// confirmationCodeMax is the exclusive upper bound for a confirmation code:
// codes run from 0 to 999999, which the six-digit zero-padded formatting turns
// into 000000-999999.
//
// [Ja] confirmationCodeMax は確認コードの排他的上限です。コードは 0 から 999999 まで
// で、6 桁ゼロ埋めの整形により 000000-999999 になります。
const confirmationCodeMax = 1000000

// GenerateConfirmationCode returns a cryptographically random six-digit numeric
// confirmation code, zero-padded so it always renders as six digits. It is the
// human-typed code emailed to verify control of an email address (e.g. during
// sign-up). It lives in auth, the secure-random utility, rather than in the
// calling UseCase so the randomness primitive stays in one place.
//
// [Ja] GenerateConfirmationCode は暗号論的乱数による 6 桁の数字確認コードを返します。
// 常に 6 桁で表示されるようゼロ埋めします。これはメールアドレスの管理権を検証する
// ため (例: サインアップ時) にメール送信され、ユーザーが入力し返すコードです。乱数
// プリミティブを 1 箇所に集約するため、呼び出し側の UseCase ではなくセキュアランダム
// ユーティリティである auth に置きます。
func GenerateConfirmationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(confirmationCodeMax))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
