package auth_test

import (
	"regexp"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// lowercaseAlnum8はリカバリーコードに求められる形である、小文字英数字ちょうど
// 8文字に一致します。
var lowercaseAlnum8 = regexp.MustCompile(`^[a-z0-9]{8}$`)

// TestGenerateRecoveryCodes_ShapeAndCountは10個のコードが返り、各コードが
// 小文字英数字8文字であることを検証します。
func TestGenerateRecoveryCodes_ShapeAndCount(t *testing.T) {
	t.Parallel()

	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes()のエラー = %v", err)
	}

	if len(codes) != auth.RecoveryCodeCount {
		t.Fatalf("len(codes) = %d、期待値 = %d", len(codes), auth.RecoveryCodeCount)
	}
	for _, code := range codes {
		if !lowercaseAlnum8.MatchString(code) {
			t.Errorf("リカバリーコード %q が小文字英数字8文字でない", code)
		}
	}
}

// TestGenerateRecoveryCodes_Uniqueは1回の生成で重複するコードが含まれないことを
// 検証します。ユーザーが同一のバックアップコードを2つ受け取らないようにするためです。
func TestGenerateRecoveryCodes_Unique(t *testing.T) {
	t.Parallel()

	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes()のエラー = %v", err)
	}

	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if _, ok := seen[code]; ok {
			t.Errorf("1回の生成でリカバリーコード %q が重複している", code)
		}
		seen[code] = struct{}{}
	}
}
