package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestTurnstileはTurnstileがCloudflareのapi.jsスクリプト (async + defer
// で読み込む) と、渡されたサイトキーを持つcf-turnstileウィジェットdivを描画し、
// サイトキーが空のとき (dev / testの無効化経路。サードパーティスクリプトも
// ウィジェットも出てはならない) は何も描画しないことを検証します。サイトキーには
// Cloudflareのダミーテストキーを使い、テストフィクスチャに留めます。Turnstileは
// 翻訳に触れないため、background contextで十分です。
func TestTurnstile(t *testing.T) {
	t.Parallel()

	// Cloudflareの「常に成功」ダミーサイトキー (テスト専用)。
	const dummySiteKey = "1x00000000000000000000AA"

	tests := []struct {
		name         string
		siteKey      string
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:    "サイトキーがあればスクリプトとウィジェットを描画する",
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
			name:      "サイトキーが空なら何も描画しない",
			siteKey:   "",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			if err := components.Turnstile(tt.siteKey).Render(context.Background(), &buf); err != nil {
				t.Fatalf("描画に失敗: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("出力 = %q、空を期待", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
		})
	}
}
