package auth_test

import (
	"encoding/base32"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/auth"
)

// totpPeriodはTOTPのタイムステップで、ドリフトのテストがタイムスタンプを
// ステップ単位でずらせるようここに写しています。
const totpPeriod = 30 * time.Second

// b32NoPaddingはauthがTOTP secretに用いるbase32エンコーディングに一致し、
// 生成されたsecretを生バイトへデコードできるようにします。
var b32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

// TestGenerateTOTPSecretは生成されたsecretが、認証アプリが期待する長さである
// 20バイト (160ビット) にデコードされる有効なbase32であることを検証します。
func TestGenerateTOTPSecret(t *testing.T) {
	t.Parallel()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret()のエラー = %v", err)
	}

	raw, err := b32NoPadding.DecodeString(secret)
	if err != nil {
		t.Fatalf("secret %q が正しいbase32でない: %v", secret, err)
	}
	if len(raw) != 20 {
		t.Errorf("secretのデコード結果のバイト数 = %d、期待値 = 20", len(raw))
	}
}

// TestGenerateTOTPSecret_Variesは生成器が単一の値に固定されていないことを確認
// します。多数の試行で2種類以上の異なるsecretを生成します。
func TestGenerateTOTPSecret_Varies(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for i := 0; i < 50; i++ {
		secret, err := auth.GenerateTOTPSecret()
		if err != nil {
			t.Fatalf("GenerateTOTPSecret()のエラー = %v", err)
		}
		seen[secret] = struct{}{}
	}
	if len(seen) < 2 {
		t.Errorf("生成されたsecretが %d 種類しかない (固定されている可能性)", len(seen))
	}
}

// TestBuildOTPAuthURLはsecretから組み立てたotpauth URIがGroobbのissuer・
// アカウントラベル・元のsecretを持つことを検証し、認証アプリが正しいキーを登録できる
// ことを確認します。生の文字列ではなくURIを解析し直して検証します。
func TestBuildOTPAuthURL(t *testing.T) {
	t.Parallel()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret()のエラー = %v", err)
	}

	url, err := auth.BuildOTPAuthURL(secret, "user@example.com")
	if err != nil {
		t.Fatalf("BuildOTPAuthURL()のエラー = %v", err)
	}

	key, err := otp.NewKeyFromURL(url)
	if err != nil {
		t.Fatalf("otpauth URI %q を解析できない: %v", url, err)
	}
	if got := key.Type(); got != "totp" {
		t.Errorf("Type() = %q、期待値 = %q", got, "totp")
	}
	if got := key.Issuer(); got != "Groobb" {
		t.Errorf("Issuer() = %q、期待値 = %q", got, "Groobb")
	}
	if got := key.AccountName(); got != "user@example.com" {
		t.Errorf("AccountName() = %q、期待値 = %q", got, "user@example.com")
	}
	if got := key.Secret(); got != secret {
		t.Errorf("Secret() = %q、期待値 = %q", got, secret)
	}
}

// TestBuildOTPAuthURL_InvalidSecretはbase32として不正なsecretが、壊れたURIを
// 生成せずエラーで拒否されることを検証します。
func TestBuildOTPAuthURL_InvalidSecret(t *testing.T) {
	t.Parallel()

	if _, err := auth.BuildOTPAuthURL("not-valid-base32-!!!", "user@example.com"); err == nil {
		t.Error("不正なsecretを渡したBuildOTPAuthURL() がエラーを返さなかった (エラーが返るべき)")
	}
}

// TestValidateTOTPCodeはドリフト方針を検証します。現在のコードと1ステップ先の
// コードは受理され (±1ステップのskew)、遠い未来のコードと不正な形式のコードは拒否され
// ます。受理・拒否の検証に次ステップと遠い未来のコードを使うのは、それらの受理可否が現在
// ステップ内での実時刻の位置に依存せず、タイムステップの境界をまたいでもテストが決定的に
// 保たれるためです。
func TestValidateTOTPCode(t *testing.T) {
	t.Parallel()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret()のエラー = %v", err)
	}

	now := time.Now()
	codeAt := func(at time.Time) string {
		code, err := totp.GenerateCode(secret, at)
		if err != nil {
			t.Fatalf("totp.GenerateCode()のエラー = %v", err)
		}
		return code
	}

	if !auth.ValidateTOTPCode(secret, codeAt(now)) {
		t.Error("ValidateTOTPCode() が現在のコードを拒否した (受理されるべき)")
	}
	if !auth.ValidateTOTPCode(secret, codeAt(now.Add(totpPeriod))) {
		t.Error("ValidateTOTPCode() が次ステップのコードを拒否した (±1ステップのskewで受理されるべき)")
	}
	if auth.ValidateTOTPCode(secret, codeAt(now.Add(10*totpPeriod))) {
		t.Error("ValidateTOTPCode() が遠い未来のコードを受理した (拒否されるべき)")
	}
	if auth.ValidateTOTPCode(secret, "notacode") {
		t.Error("ValidateTOTPCode() が不正な形式のコードを受理した (拒否されるべき)")
	}
	if auth.ValidateTOTPCode("not-valid-base32-!!!", codeAt(now)) {
		t.Error("ValidateTOTPCode() が解析できないsecretに対してコードを受理した (拒否されるべき)")
	}
}
