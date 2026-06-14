package auth_test

import (
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
