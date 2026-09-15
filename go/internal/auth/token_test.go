package auth_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
)

// TestGenerateSecureTokenは、トークンが空でないこと、連続呼び出しが異なる値を
// 返すこと (実際に乱数であること)、そして固定長であること (24バイトの乱数はbase64url
// で32文字になる) を検証します。
func TestGenerateSecureToken(t *testing.T) {
	t.Parallel()

	token1, err := auth.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken()のエラー = %v", err)
	}
	if token1 == "" {
		t.Fatal("GenerateSecureToken()が空のトークンを返した")
	}
	if len(token1) != 32 {
		t.Errorf("len(token) = %d、期待値 = 32", len(token1))
	}

	token2, err := auth.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken()のエラー = %v", err)
	}
	if token1 == token2 {
		t.Error("GenerateSecureToken()が同じトークンを2回返した (ランダムであるべき)")
	}
}

// TestHashTokenは、HashTokenが決定的であること (同じトークンは常に同じ
// ダイジェストになり、保存済みダイジェストを完全一致のルックアップで照合できる)、
// SHA-256ダイジェストの固定64文字16進形式を生成すること、そして異なるトークンが
// 異なるダイジェストになることを検証します。
func TestHashToken(t *testing.T) {
	t.Parallel()

	const token = "an-opaque-reset-token"

	digest := auth.HashToken(token)
	if len(digest) != 64 {
		t.Errorf("len(digest) = %d、期待値 = 64 (16進数で表したSHA-256)", len(digest))
	}
	if again := auth.HashToken(token); again != digest {
		t.Errorf("HashTokenの結果が決定的でない: %q != %q", again, digest)
	}
	if other := auth.HashToken("a-different-token"); other == digest {
		t.Error("HashTokenが異なるトークンに同じダイジェストを返した")
	}
}
