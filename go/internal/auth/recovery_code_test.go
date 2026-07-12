package auth_test

import (
	"regexp"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// lowercaseAlnum8 matches exactly eight lowercase-alphanumeric characters, the
// required shape of a recovery code.
//
// [Ja] lowercaseAlnum8 はリカバリーコードに求められる形である、小文字英数字ちょうど
// 8 文字に一致します。
var lowercaseAlnum8 = regexp.MustCompile(`^[a-z0-9]{8}$`)

// TestGenerateRecoveryCodes_ShapeAndCount verifies that ten codes are returned and
// each is eight lowercase-alphanumeric characters.
//
// [Ja] TestGenerateRecoveryCodes_ShapeAndCount は 10 個のコードが返り、各コードが
// 小文字英数字 8 文字であることを検証します。
func TestGenerateRecoveryCodes_ShapeAndCount(t *testing.T) {
	t.Parallel()

	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error = %v", err)
	}

	if len(codes) != auth.RecoveryCodeCount {
		t.Fatalf("len(codes) = %d, want %d", len(codes), auth.RecoveryCodeCount)
	}
	for _, code := range codes {
		if !lowercaseAlnum8.MatchString(code) {
			t.Errorf("リカバリーコード %q が小文字英数字 8 文字でない", code)
		}
	}
}

// TestGenerateRecoveryCodes_Unique verifies that a single batch does not contain
// duplicate codes, so a user never receives two identical backup codes.
//
// [Ja] TestGenerateRecoveryCodes_Unique は 1 回の生成で重複するコードが含まれないことを
// 検証します。ユーザーが同一のバックアップコードを 2 つ受け取らないようにするためです。
func TestGenerateRecoveryCodes_Unique(t *testing.T) {
	t.Parallel()

	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error = %v", err)
	}

	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if _, ok := seen[code]; ok {
			t.Errorf("1 回の生成でリカバリーコード %q が重複している", code)
		}
		seen[code] = struct{}{}
	}
}
