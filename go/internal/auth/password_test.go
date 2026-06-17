package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// TestHashPasswordAndCheckPassword verifies that a hash produced by
// HashPassword validates against the original password and rejects a wrong one.
//
// [Ja] TestHashPasswordAndCheckPassword は HashPassword が生成したハッシュが元の
// パスワードで検証でき、誤ったパスワードを拒否することを検証します。
func TestHashPasswordAndCheckPassword(t *testing.T) {
	t.Parallel()

	const plain = "correct horse battery staple"

	hash, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == plain {
		t.Error("HashPassword() returned the plaintext instead of a hash")
	}

	if err := auth.CheckPassword(hash, plain); err != nil {
		t.Errorf("CheckPassword() with the correct password error = %v, want nil", err)
	}
	if err := auth.CheckPassword(hash, "wrong-password"); err == nil {
		t.Error("CheckPassword() with a wrong password = nil, want an error")
	}
}

// TestHashPasswordProducesUniqueHashes verifies that bcrypt salting makes two
// hashes of the same password differ while both still validate.
//
// [Ja] TestHashPasswordProducesUniqueHashes は bcrypt のソルトにより同じパスワード
// の 2 つのハッシュが異なり、かつどちらも検証できることを確認します。
func TestHashPasswordProducesUniqueHashes(t *testing.T) {
	t.Parallel()

	const plain = "same-input"

	hash1, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	hash2, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes; bcrypt salting is not working")
	}
	if err := auth.CheckPassword(hash1, plain); err != nil {
		t.Errorf("CheckPassword(hash1) error = %v", err)
	}
	if err := auth.CheckPassword(hash2, plain); err != nil {
		t.Errorf("CheckPassword(hash2) error = %v", err)
	}
}

// TestValidatePasswordStrength verifies the length policy: a password shorter
// than the minimum returns ErrPasswordTooShort, one longer than the byte maximum
// returns ErrPasswordTooLong, and a password within range passes. The minimum is
// measured in runes, so an 8-character Japanese password (24 bytes) is accepted,
// and the maximum is measured in bytes to honor bcrypt's 72-byte limit.
//
// [Ja] TestValidatePasswordStrength は長さポリシーを検証します。最小未満は
// ErrPasswordTooShort、バイト最大超過は ErrPasswordTooLong を返し、範囲内のパスワードは
// 通ります。最小は rune 単位で測るため 8 文字の日本語パスワード (24 バイト) は受理され、
// 最大は bcrypt の 72 バイト制限を尊重してバイト単位で測ります。
func TestValidatePasswordStrength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "valid ASCII password", password: "password123", wantErr: nil},
		{name: "exactly the minimum length", password: "12345678", wantErr: nil},
		{name: "Japanese password of 7 runes is too short", password: "ぱすわーどです", wantErr: auth.ErrPasswordTooShort},
		{name: "Japanese password of 8 runes accepted", password: "ぱすわーどですよ", wantErr: nil},
		{name: "too short", password: "1234567", wantErr: auth.ErrPasswordTooShort},
		{name: "empty", password: "", wantErr: auth.ErrPasswordTooShort},
		{name: "too long (73 bytes)", password: strings.Repeat("a", 73), wantErr: auth.ErrPasswordTooLong},
		{name: "exactly the maximum byte length", password: strings.Repeat("a", 72), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := auth.ValidatePasswordStrength(tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePasswordStrength(%q) error = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}
