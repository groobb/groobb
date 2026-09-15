package templates_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
)

// TestTはtemplヘルパーがi18n.Tに委譲し、contextのロケールで翻訳する
// ことを検証する。
func TestT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    model.Locale
		messageID string
		want      string
	}{
		{name: "日本語", locale: model.LocaleJa, messageID: "default_description", want: "Groobbは掲示板サービスです。"},
		{name: "英語", locale: model.LocaleEn, messageID: "default_description", want: "Groobb is a bulletin board service."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			if got := templates.T(ctx, tt.messageID); got != tt.want {
				t.Errorf("T(%q) = %q、期待値 = %q", tt.messageID, got, tt.want)
			}
		})
	}
}

func TestLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(ctx context.Context) context.Context
		want  string
	}{
		{name: "英語が設定されている", setup: func(ctx context.Context) context.Context { return i18n.SetLocale(ctx, model.LocaleEn) }, want: string(model.LocaleEn)},
		{name: "何も設定されていなければ既定値にフォールバックする", setup: func(ctx context.Context) context.Context { return ctx }, want: string(model.DefaultLocale)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.setup(context.Background())
			if got := templates.Locale(ctx); got != tt.want {
				t.Errorf("Locale() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}
