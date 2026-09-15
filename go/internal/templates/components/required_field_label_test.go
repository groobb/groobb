package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestRequiredFieldLabelはRequiredFieldLabelがfor="{field}" でラベルを入力欄へ
// 結び付け、呼び出し側のラベル文言をそのまま描画し、現在のロケールでローカライズした必須
// マーカーを添えることを検証します。マーカーをロケール別に確認するのは、それが本
// コンポーネント自身の (唯一保持する) 文言だからです。翻訳が欠けると、すべてのフォームに
// 一度に素のメッセージIDが現れることになります。
func TestRequiredFieldLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		locale       model.Locale
		wantContains []string
	}{
		{
			name:   "日本語",
			locale: model.LocaleJa,
			wantContains: []string{
				`<label for="email"`,
				"メールアドレス",
				"必須",
			},
		},
		{
			name:   "英語",
			locale: model.LocaleEn,
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
				t.Fatalf("Render()のエラー = %v", err)
			}

			got := buf.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("描画されたラベルに %q が含まれていない\n実測値: %s", want, got)
				}
			}
		})
	}
}
