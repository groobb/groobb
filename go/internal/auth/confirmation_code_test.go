package auth_test

import (
	"regexp"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// sixDigitsは確認コードに求められる形であるASCII数字6桁ちょうどに一致します。
var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

// TestGenerateConfirmationCode_Formatは生成される各コードが、ゼロ埋めされた値も
// 含めて6桁の数字であることを多数の試行で検証します。先頭ゼロの欠落や範囲外の値が
// あれば検出されます。
func TestGenerateConfirmationCode_Format(t *testing.T) {
	t.Parallel()

	for i := 0; i < 1000; i++ {
		code, err := auth.GenerateConfirmationCode()
		if err != nil {
			t.Fatalf("GenerateConfirmationCode()のエラー = %v", err)
		}
		if !sixDigits.MatchString(code) {
			t.Fatalf("GenerateConfirmationCode() = %q、期待値は6桁の数字", code)
		}
	}
}

// TestGenerateConfirmationCode_Variesは生成器が単一の値に固定されていないことを
// 確認します。多数の試行で2種類以上の異なるコードを生成します。
func TestGenerateConfirmationCode_Varies(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		code, err := auth.GenerateConfirmationCode()
		if err != nil {
			t.Fatalf("GenerateConfirmationCode()のエラー = %v", err)
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 2 {
		t.Errorf("生成されたコードが %d 種類しかない (固定されている可能性)", len(seen))
	}
}
