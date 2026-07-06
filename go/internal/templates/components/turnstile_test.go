package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestTurnstile verifies that Turnstile renders the Cloudflare api.js script
// (loaded async + defer) and the cf-turnstile widget div carrying the given site
// key, and renders nothing at all when the site key is empty (the dev / test
// disable path, where neither the third-party script nor the widget should
// appear). The site key uses Cloudflare's dummy testing key, kept to test
// fixtures. Turnstile touches no translations, so a background context is enough.
//
// [Ja] TestTurnstile は Turnstile が Cloudflare の api.js スクリプト (async + defer
// で読み込む) と、渡されたサイトキーを持つ cf-turnstile ウィジェット div を描画し、
// サイトキーが空のとき (dev / test の無効化経路。サードパーティスクリプトも
// ウィジェットも出てはならない) は何も描画しないことを検証します。サイトキーには
// Cloudflare のダミーテストキーを使い、テストフィクスチャに留めます。Turnstile は
// 翻訳に触れないため、background context で十分です。
func TestTurnstile(t *testing.T) {
	t.Parallel()

	// Cloudflare's always-passing dummy site key, kept to test fixtures.
	//
	// [Ja] Cloudflare の「常に成功」ダミーサイトキー (テスト専用)。
	const dummySiteKey = "1x00000000000000000000AA"

	tests := []struct {
		name         string
		siteKey      string
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:    "with site key renders script and widget",
			siteKey: dummySiteKey,
			wantContains: []string{
				`<script src="https://challenges.cloudflare.com/turnstile/v0/api.js"`,
				"async",
				"defer",
				`<div class="cf-turnstile"`,
				`data-sitekey="1x00000000000000000000AA"`,
			},
		},
		{
			name:      "empty site key renders nothing",
			siteKey:   "",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			if err := components.Turnstile(tt.siteKey).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render failed: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("expected no output, got %q", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q\noutput: %s", want, got)
				}
			}
		})
	}
}
