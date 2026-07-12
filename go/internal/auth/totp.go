package auth

import (
	"crypto/rand"
	"encoding/base32"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	// totpIssuer is the issuer shown in authenticator apps and encoded in the
	// otpauth URI, identifying the enrolled account as belonging to Groobb.
	//
	// [Ja] totpIssuer は認証アプリに表示され otpauth URI に埋め込まれる issuer で、
	// 登録されるアカウントが Groobb のものであることを示します。
	totpIssuer = "Groobb"

	// totpPeriod is the TOTP time step in seconds. 30 seconds is the value
	// authenticator apps default to, keeping codes interchangeable with them.
	//
	// [Ja] totpPeriod は TOTP のタイムステップ (秒) です。認証アプリの既定値である
	// 30 秒とし、コードを認証アプリと相互に使えるようにします。
	totpPeriod uint = 30

	// totpSkew allows one time step on either side of the current one when
	// validating a code, tolerating clock drift of up to totpPeriod seconds in
	// each direction without widening the acceptance window further.
	//
	// [Ja] totpSkew はコード検証時に現在のタイムステップの前後 1 ステップを許容し、
	// 受理窓をこれ以上広げずに各方向 totpPeriod 秒までの時刻ドリフトを許容します。
	totpSkew uint = 1

	// totpSecretBytes is the size of a generated TOTP secret. 20 bytes (160 bits)
	// matches the pquerna/otp and authenticator-app default and base32-encodes to
	// a 32-character secret.
	//
	// [Ja] totpSecretBytes は生成する TOTP secret のバイト数です。20 バイト (160 ビット)
	// は pquerna/otp と認証アプリの既定に一致し、base32 で 32 文字の secret になります。
	totpSecretBytes = 20
)

const (
	// totpDigits and totpAlgorithm are the code shape (six digits) and HMAC
	// algorithm (SHA1) authenticator apps default to; both generation and
	// validation must agree on them for codes to match.
	//
	// [Ja] totpDigits と totpAlgorithm はコードの形 (6 桁) と HMAC アルゴリズム (SHA1)
	// で、認証アプリの既定値です。コードが一致するには生成側と検証側が両方揃える必要が
	// あります。
	totpDigits    = otp.DigitsSix
	totpAlgorithm = otp.AlgorithmSHA1
)

// totpSecretEncoding is the base32 encoding pquerna/otp uses for TOTP secrets
// (standard alphabet, no padding). Encoding a generated secret with it lets the
// same secret round-trip through BuildOTPAuthURL and ValidateTOTPCode.
//
// [Ja] totpSecretEncoding は pquerna/otp が TOTP secret に用いる base32 エンコーディング
// (標準アルファベット・パディング無し) です。生成した secret をこれでエンコードすることで、
// 同じ secret が BuildOTPAuthURL と ValidateTOTPCode を通じて往復できます。
var totpSecretEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret returns a cryptographically random, base32-encoded TOTP
// shared secret. The secret is what the authenticator app and the server both
// derive time-based codes from; it is stored per user and turned into a scannable
// otpauth URI by BuildOTPAuthURL. It uses the base32 (no padding) encoding
// pquerna/otp expects so ValidateTOTPCode can verify codes against it directly,
// and lives in auth (the secure-random utility) so the randomness primitive stays
// in one place.
//
// [Ja] GenerateTOTPSecret は暗号論的乱数による base32 エンコード済みの TOTP 共有
// シークレットを返します。secret は認証アプリとサーバーの双方が時刻ベースのコードを
// 導出する元で、ユーザーごとに保存し、BuildOTPAuthURL でスキャン可能な otpauth URI に
// します。ValidateTOTPCode がそのままコードを検証できるよう pquerna/otp が期待する
// base32 (パディング無し) エンコーディングを用い、乱数プリミティブを 1 箇所に集約する
// ためセキュアランダムユーティリティである auth に置きます。
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, totpSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return totpSecretEncoding.EncodeToString(b), nil
}

// BuildOTPAuthURL builds the otpauth:// URI for an existing base32 secret, using
// "Groobb" as the issuer and accountName (the user's email) as the label. The URI
// is what the setting page encodes into a QR code so an authenticator app can
// enroll the secret. It rebuilds the URI from a stored secret rather than
// generating a new one, so re-rendering the enrollment form keeps the same secret.
//
// [Ja] BuildOTPAuthURL は既存の base32 secret に対する otpauth:// URI を、issuer に
// "Groobb"、ラベルに accountName (ユーザーの email) を使って組み立てます。この URI は
// 設定画面が QR コードにエンコードし、認証アプリが secret を登録できるようにするものです。
// 新しい secret を生成するのではなく保存済みの secret から URI を組み直すため、登録
// フォームを再描画しても同じ secret が保たれます。
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

// ValidateTOTPCode reports whether code is a valid TOTP code for secret at the
// current time, allowing totpSkew time steps of drift on either side. It returns
// false on any malformed input (a bad code or an unparsable secret) rather than
// surfacing an error, since the caller only needs the accept/reject decision.
//
// [Ja] ValidateTOTPCode は code が現在時刻において secret に対する有効な TOTP コードか
// を、前後 totpSkew タイムステップのドリフトを許容して返します。呼び出し側は受理か拒否かの
// 判断だけを必要とするため、不正な入力 (誤ったコードや解析できない secret) ではエラーを
// 返さず false を返します。
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
