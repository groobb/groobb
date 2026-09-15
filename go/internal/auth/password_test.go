package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// TestHashPasswordAndCheckPasswordはHashPasswordが生成したハッシュが元の
// パスワードで検証でき、誤ったパスワードを拒否することを検証します。
func TestHashPasswordAndCheckPassword(t *testing.T) {
	t.Parallel()

	const plain = "correct horse battery staple"

	hash, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}
	if hash == plain {
		t.Error("HashPassword()がハッシュではなく平文を返した")
	}

	if err := auth.CheckPassword(hash, plain); err != nil {
		t.Errorf("正しいパスワードでのCheckPassword()のエラー = %v、期待値 = nil", err)
	}
	if err := auth.CheckPassword(hash, "wrong-password"); err == nil {
		t.Error("誤ったパスワードでのCheckPassword() = nil、エラーを期待")
	}
}

// TestHashPasswordProducesUniqueHashesはbcryptのソルトにより同じパスワード
// の2つのハッシュが異なり、かつどちらも検証できることを確認します。
func TestHashPasswordProducesUniqueHashes(t *testing.T) {
	t.Parallel()

	const plain = "same-input"

	hash1, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}
	hash2, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword()が同一のハッシュを生成した (bcryptのソルトが機能していない)")
	}
	if err := auth.CheckPassword(hash1, plain); err != nil {
		t.Errorf("CheckPassword(hash1)のエラー = %v", err)
	}
	if err := auth.CheckPassword(hash2, plain); err != nil {
		t.Errorf("CheckPassword(hash2)のエラー = %v", err)
	}
}

// TestValidatePasswordStrengthは長さポリシーを検証します。最小未満は
// ErrPasswordTooShort、バイト最大超過はErrPasswordTooLongを返し、範囲内のパスワードは
// 通ります。最小はrune単位で測るため8文字の日本語パスワード (24バイト) は受理され、
// 最大はbcryptの72バイト制限を尊重してバイト単位で測ります。
func TestValidatePasswordStrength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "有効なASCIIのパスワード", password: "password123", wantErr: nil},
		{name: "最小の長さちょうど", password: "12345678", wantErr: nil},
		{name: "7文字の日本語パスワードは短すぎる", password: "ぱすわーどです", wantErr: auth.ErrPasswordTooShort},
		{name: "8文字の日本語パスワードは受理される", password: "ぱすわーどですよ", wantErr: nil},
		{name: "短すぎる", password: "1234567", wantErr: auth.ErrPasswordTooShort},
		{name: "空", password: "", wantErr: auth.ErrPasswordTooShort},
		{name: "長すぎる (73バイト)", password: strings.Repeat("a", 73), wantErr: auth.ErrPasswordTooLong},
		{name: "最大バイト長ちょうど", password: strings.Repeat("a", 72), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := auth.ValidatePasswordStrength(tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePasswordStrength(%q)のエラー = %v、期待値 = %v", tt.password, err, tt.wantErr)
			}
		})
	}
}
