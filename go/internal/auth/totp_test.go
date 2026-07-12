package auth_test

import (
	"encoding/base32"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/auth"
)

// totpPeriod is the TOTP time step, mirrored here so drift tests can shift a
// timestamp by whole steps.
//
// [Ja] totpPeriod は TOTP のタイムステップで、ドリフトのテストがタイムスタンプを
// ステップ単位でずらせるようここに写しています。
const totpPeriod = 30 * time.Second

// b32NoPadding matches the base32 encoding auth uses for TOTP secrets, so a
// generated secret can be decoded back to its raw bytes.
//
// [Ja] b32NoPadding は auth が TOTP secret に用いる base32 エンコーディングに一致し、
// 生成された secret を生バイトへデコードできるようにします。
var b32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

// TestGenerateTOTPSecret verifies that a generated secret is valid base32 that
// decodes to 20 bytes (160 bits), the length authenticator apps expect.
//
// [Ja] TestGenerateTOTPSecret は生成された secret が、認証アプリが期待する長さである
// 20 バイト (160 ビット) にデコードされる有効な base32 であることを検証します。
func TestGenerateTOTPSecret(t *testing.T) {
	t.Parallel()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error = %v", err)
	}

	raw, err := b32NoPadding.DecodeString(secret)
	if err != nil {
		t.Fatalf("secret %q is not valid base32: %v", secret, err)
	}
	if len(raw) != 20 {
		t.Errorf("secret decoded to %d bytes, want 20", len(raw))
	}
}

// TestGenerateTOTPSecret_Varies checks that the generator is not stuck on a single
// value: across many draws it produces more than one distinct secret.
//
// [Ja] TestGenerateTOTPSecret_Varies は生成器が単一の値に固定されていないことを確認
// します。多数の試行で 2 種類以上の異なる secret を生成します。
func TestGenerateTOTPSecret_Varies(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for i := 0; i < 50; i++ {
		secret, err := auth.GenerateTOTPSecret()
		if err != nil {
			t.Fatalf("GenerateTOTPSecret() error = %v", err)
		}
		seen[secret] = struct{}{}
	}
	if len(seen) < 2 {
		t.Errorf("生成された secret が %d 種類しかない (固定されている可能性)", len(seen))
	}
}

// TestBuildOTPAuthURL verifies that the otpauth URI built from a secret carries
// the Groobb issuer, the account label, and the original secret, so an
// authenticator app enrolls the right key. It parses the URI back rather than
// asserting on the raw string.
//
// [Ja] TestBuildOTPAuthURL は secret から組み立てた otpauth URI が Groobb の issuer・
// アカウントラベル・元の secret を持つことを検証し、認証アプリが正しいキーを登録できる
// ことを確認します。生の文字列ではなく URI を解析し直して検証します。
func TestBuildOTPAuthURL(t *testing.T) {
	t.Parallel()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error = %v", err)
	}

	url, err := auth.BuildOTPAuthURL(secret, "user@example.com")
	if err != nil {
		t.Fatalf("BuildOTPAuthURL() error = %v", err)
	}

	key, err := otp.NewKeyFromURL(url)
	if err != nil {
		t.Fatalf("otpauth URI %q is not parsable: %v", url, err)
	}
	if got := key.Type(); got != "totp" {
		t.Errorf("Type() = %q, want %q", got, "totp")
	}
	if got := key.Issuer(); got != "Groobb" {
		t.Errorf("Issuer() = %q, want %q", got, "Groobb")
	}
	if got := key.AccountName(); got != "user@example.com" {
		t.Errorf("AccountName() = %q, want %q", got, "user@example.com")
	}
	if got := key.Secret(); got != secret {
		t.Errorf("Secret() = %q, want %q", got, secret)
	}
}

// TestBuildOTPAuthURL_InvalidSecret verifies that a secret that is not valid
// base32 is rejected with an error rather than producing a broken URI.
//
// [Ja] TestBuildOTPAuthURL_InvalidSecret は base32 として不正な secret が、壊れた URI を
// 生成せずエラーで拒否されることを検証します。
func TestBuildOTPAuthURL_InvalidSecret(t *testing.T) {
	t.Parallel()

	if _, err := auth.BuildOTPAuthURL("not-valid-base32-!!!", "user@example.com"); err == nil {
		t.Error("不正な secret を渡した BuildOTPAuthURL() がエラーを返さなかった (エラーが返るべき)")
	}
}

// TestValidateTOTPCode verifies the drift policy: the current code and the code
// one step in the future are accepted (the ±1 step skew), while a far-future code
// and a malformed code are rejected. The next-step and far-future codes are used
// for the accept/reject assertions because their acceptance does not depend on
// where the wall clock sits within the current step, keeping the test
// deterministic across time-step boundaries.
//
// [Ja] TestValidateTOTPCode はドリフト方針を検証します。現在のコードと 1 ステップ先の
// コードは受理され (±1 ステップの skew)、遠い未来のコードと不正な形式のコードは拒否され
// ます。受理・拒否の検証に次ステップと遠い未来のコードを使うのは、それらの受理可否が現在
// ステップ内での実時刻の位置に依存せず、タイムステップの境界をまたいでもテストが決定的に
// 保たれるためです。
func TestValidateTOTPCode(t *testing.T) {
	t.Parallel()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error = %v", err)
	}

	now := time.Now()
	codeAt := func(at time.Time) string {
		code, err := totp.GenerateCode(secret, at)
		if err != nil {
			t.Fatalf("totp.GenerateCode() error = %v", err)
		}
		return code
	}

	if !auth.ValidateTOTPCode(secret, codeAt(now)) {
		t.Error("ValidateTOTPCode() が現在のコードを拒否した (受理されるべき)")
	}
	if !auth.ValidateTOTPCode(secret, codeAt(now.Add(totpPeriod))) {
		t.Error("ValidateTOTPCode() が次ステップのコードを拒否した (±1 ステップの skew で受理されるべき)")
	}
	if auth.ValidateTOTPCode(secret, codeAt(now.Add(10*totpPeriod))) {
		t.Error("ValidateTOTPCode() が遠い未来のコードを受理した (拒否されるべき)")
	}
	if auth.ValidateTOTPCode(secret, "notacode") {
		t.Error("ValidateTOTPCode() が不正な形式のコードを受理した (拒否されるべき)")
	}
	if auth.ValidateTOTPCode("not-valid-base32-!!!", codeAt(now)) {
		t.Error("ValidateTOTPCode() が解析できない secret に対してコードを受理した (拒否されるべき)")
	}
}
