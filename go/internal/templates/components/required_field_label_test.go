package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestRequiredFieldLabel verifies that RequiredFieldLabel binds the label to the
// control through for="{field}", renders the caller's label text as given, and
// appends the required marker localized for the current locale. The marker is
// checked per locale because it is the component's own wording (the only text it
// owns), so a missing translation would otherwise surface as the raw message ID
// in every form at once.
//
// [Ja] TestRequiredFieldLabel は RequiredFieldLabel が for="{field}" でラベルを入力欄へ
// 結び付け、呼び出し側のラベル文言をそのまま描画し、現在のロケールでローカライズした必須
// マーカーを添えることを検証します。マーカーをロケール別に確認するのは、それが本
// コンポーネント自身の (唯一保持する) 文言だからです。翻訳が欠けると、すべてのフォームに
// 一度に素のメッセージ ID が現れることになります。
func TestRequiredFieldLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		locale       string
		wantContains []string
	}{
		{
			name:   "Japanese",
			locale: i18n.LangJa,
			wantContains: []string{
				`<label for="email"`,
				"メールアドレス",
				"必須",
			},
		},
		{
			name:   "English",
			locale: i18n.LangEn,
			wantContains: []string{
				`<label for="email"`,
				"メールアドレス",
				"Required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			var buf strings.Builder
			if err := components.RequiredFieldLabel("email", "メールアドレス").Render(ctx, &buf); err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			got := buf.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("rendered label does not contain %q\ngot: %s", want, got)
				}
			}
		})
	}
}
