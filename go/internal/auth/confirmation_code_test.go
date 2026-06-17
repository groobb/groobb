package auth_test

import (
	"regexp"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// sixDigits matches exactly six ASCII digits, the required shape of a
// confirmation code.
//
// [Ja] sixDigits は確認コードに求められる形である ASCII 数字 6 桁ちょうどに一致します。
var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

// TestGenerateConfirmationCode_Format verifies that every generated code is six
// numeric digits, including zero-padded values, across many draws so a missing
// leading zero or out-of-range value would be caught.
//
// [Ja] TestGenerateConfirmationCode_Format は生成される各コードが、ゼロ埋めされた値も
// 含めて 6 桁の数字であることを多数の試行で検証します。先頭ゼロの欠落や範囲外の値が
// あれば検出されます。
func TestGenerateConfirmationCode_Format(t *testing.T) {
	t.Parallel()

	for i := 0; i < 1000; i++ {
		code, err := auth.GenerateConfirmationCode()
		if err != nil {
			t.Fatalf("GenerateConfirmationCode() error = %v", err)
		}
		if !sixDigits.MatchString(code) {
			t.Fatalf("GenerateConfirmationCode() = %q, want six digits", code)
		}
	}
}

// TestGenerateConfirmationCode_Varies checks that the generator is not stuck on
// a single value: across many draws it produces more than one distinct code.
//
// [Ja] TestGenerateConfirmationCode_Varies は生成器が単一の値に固定されていないことを
// 確認します。多数の試行で 2 種類以上の異なるコードを生成します。
func TestGenerateConfirmationCode_Varies(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		code, err := auth.GenerateConfirmationCode()
		if err != nil {
			t.Fatalf("GenerateConfirmationCode() error = %v", err)
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 2 {
		t.Errorf("生成されたコードが %d 種類しかない (固定されている可能性)", len(seen))
	}
}
