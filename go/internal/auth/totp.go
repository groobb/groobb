package auth

import (
	"crypto/rand"
	"encoding/base32"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	// totpIssuerは認証アプリに表示されotpauth URIに埋め込まれるissuerで、
	// 登録されるアカウントがGroobbのものであることを示します。
	totpIssuer = "Groobb"

	// totpPeriodはTOTPのタイムステップ (秒) です。認証アプリの既定値である
	// 30秒とし、コードを認証アプリと相互に使えるようにします。
	totpPeriod uint = 30

	// totpSkewはコード検証時に現在のタイムステップの前後1ステップを許容し、
	// 受理窓をこれ以上広げずに各方向totpPeriod秒までの時刻ドリフトを許容します。
	totpSkew uint = 1

	// totpSecretBytesは生成するTOTP secretのバイト数です。20バイト (160ビット)
	// はpquerna/otpと認証アプリの既定に一致し、base32で32文字のsecretになります。
	totpSecretBytes = 20
)

const (
	// totpDigitsとtotpAlgorithmはコードの形 (6桁) とHMACアルゴリズム (SHA1)
	// で、認証アプリの既定値です。コードが一致するには生成側と検証側が両方揃える必要が
	// あります。
	totpDigits    = otp.DigitsSix
	totpAlgorithm = otp.AlgorithmSHA1
)

// totpSecretEncodingはpquerna/otpがTOTP secretに用いるbase32エンコーディング
// (標準アルファベット・パディング無し) です。生成したsecretをこれでエンコードすることで、
// 同じsecretがBuildOTPAuthURLとValidateTOTPCodeを通じて往復できます。
var totpSecretEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecretは暗号論的乱数によるbase32エンコード済みのTOTP共有
// シークレットを返します。secretは認証アプリとサーバーの双方が時刻ベースのコードを
// 導出する元で、ユーザーごとに保存し、BuildOTPAuthURLでスキャン可能なotpauth URIに
// します。ValidateTOTPCodeがそのままコードを検証できるようpquerna/otpが期待する
// base32 (パディング無し) エンコーディングを用い、乱数プリミティブを1箇所に集約する
// ためセキュアランダムユーティリティであるauthに置きます。
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, totpSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return totpSecretEncoding.EncodeToString(b), nil
}

// BuildOTPAuthURLは既存のbase32 secretに対するotpauth:// URIを、issuerに
// "Groobb"、ラベルにaccountName (ユーザーのemail) を使って組み立てます。このURIは
// 設定画面がQRコードにエンコードし、認証アプリがsecretを登録できるようにするものです。
// 新しいsecretを生成するのではなく保存済みのsecretからURIを組み直すため、登録
// フォームを再描画しても同じsecretが保たれます。
func BuildOTPAuthURL(secret, accountName string) (string, error) {
	raw, err := totpSecretEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountName,
		Period:      totpPeriod,
		Digits:      totpDigits,
		Algorithm:   totpAlgorithm,
		Secret:      raw,
	})
	if err != nil {
		return "", err
	}
	return key.URL(), nil
}

// ValidateTOTPCodeはcodeが現在時刻においてsecretに対する有効なTOTPコードか
// を、前後totpSkewタイムステップのドリフトを許容して返します。呼び出し側は受理か拒否かの
// 判断だけを必要とするため、不正な入力 (誤ったコードや解析できないsecret) ではエラーを
// 返さずfalseを返します。
func ValidateTOTPCode(secret, code string) bool {
	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    totpPeriod,
		Skew:      totpSkew,
		Digits:    totpDigits,
		Algorithm: totpAlgorithm,
	})
	if err != nil {
		return false
	}
	return valid
}
