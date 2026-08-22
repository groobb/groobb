package middleware_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/middleware"
)

// TestSanitizeReturnTo verifies that only a same-origin relative path survives:
// a value naming another origin outright, or one a browser would read as naming
// another origin, is dropped so it can never reach a Location header. It also
// pins the normalization of an accepted value (fragment dropped, path escaped).
//
// [Ja] TestSanitizeReturnTo は同一オリジンの相対パスだけが通ることを検証する。別オリジンを
// 明示的に指す値も、ブラウザが別オリジン指定として解釈する値も破棄され、Location ヘッダーに
// 到達しないことを確認する。併せて、受け付けた値の正規化 (フラグメントの除去・パスの
// エスケープ) も固定する。
func TestSanitizeReturnTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "相対パス", raw: "/settings", want: "/settings"},
		{name: "クエリ付きの相対パス", raw: "/settings?from=home", want: "/settings?from=home"},
		{name: "ルート", raw: "/", want: "/"},
		{name: "フラグメントは落とす", raw: "/settings#email", want: "/settings"},
		{name: "エスケープが必要な文字はエスケープする", raw: "/settings/email edit", want: "/settings/email%20edit"},
		{name: "空文字", raw: "", want: ""},
		{name: "スラッシュ始まりでない", raw: "settings", want: ""},
		{name: "プロトコル相対 URL", raw: "//evil.example.com/settings", want: ""},
		{name: "バックスラッシュ始まり (ブラウザはプロトコル相対として解釈する)", raw: `/\evil.example.com`, want: ""},
		{name: "スキーム付き絶対 URL", raw: "https://evil.example.com/settings", want: ""},
		{name: "javascript スキーム", raw: "javascript:alert(1)", want: ""},
		{name: "制御文字を含む", raw: "/settings\nLocation: https://evil.example.com", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := middleware.SanitizeReturnTo(tt.raw); got != tt.want {
				t.Errorf("SanitizeReturnTo(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
