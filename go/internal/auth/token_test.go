package auth_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// TestGenerateSecureToken verifies that the token is non-empty, that successive
// calls return different values (so it is actually random), and that it is a
// fixed length (24 random bytes base64url-encode to 32 characters).
//
// [Ja] TestGenerateSecureToken は、トークンが空でないこと、連続呼び出しが異なる値を
// 返すこと (実際に乱数であること)、そして固定長であること (24 バイトの乱数は base64url
// で 32 文字になる) を検証します。
func TestGenerateSecureToken(t *testing.T) {
	t.Parallel()

	token1, err := auth.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken() error = %v", err)
	}
	if token1 == "" {
		t.Fatal("GenerateSecureToken() returned an empty token")
	}
	if len(token1) != 32 {
		t.Errorf("len(token) = %d, want 32", len(token1))
	}

	token2, err := auth.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken() error = %v", err)
	}
	if token1 == token2 {
		t.Error("GenerateSecureToken() returned the same token twice; it should be random")
	}
}

// TestHashToken verifies that HashToken is deterministic (the same token always
// hashes to the same digest, so a stored digest can be matched by an exact
// lookup), that it produces the fixed 64-character hex form of a SHA-256 digest,
// and that different tokens hash to different digests.
//
// [Ja] TestHashToken は、HashToken が決定的であること (同じトークンは常に同じ
// ダイジェストになり、保存済みダイジェストを完全一致のルックアップで照合できる)、
// SHA-256 ダイジェストの固定 64 文字 16 進形式を生成すること、そして異なるトークンが
// 異なるダイジェストになることを検証します。
func TestHashToken(t *testing.T) {
	t.Parallel()

	const token = "an-opaque-reset-token"

	digest := auth.HashToken(token)
	if len(digest) != 64 {
		t.Errorf("len(digest) = %d, want 64 (hex-encoded SHA-256)", len(digest))
	}
	if again := auth.HashToken(token); again != digest {
		t.Errorf("HashToken is not deterministic: %q != %q", again, digest)
	}
	if other := auth.HashToken("a-different-token"); other == digest {
		t.Error("HashToken returned the same digest for different tokens")
	}
}
