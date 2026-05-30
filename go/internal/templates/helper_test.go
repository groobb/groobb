package templates_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates"
)

// TestT verifies that the templ helper delegates to i18n.T and translates using
// the locale stored in the context.
//
// [Ja] TestT は templ ヘルパーが i18n.T に委譲し、context のロケールで翻訳する
// ことを検証する。
func TestT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    string
		messageID string
		want      string
	}{
		{name: "Japanese", locale: i18n.LangJa, messageID: "default_description", want: "Groobb は掲示板サービスです。"},
		{name: "English", locale: i18n.LangEn, messageID: "default_description", want: "Groobb is a bulletin board service."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			if got := templates.T(ctx, tt.messageID); got != tt.want {
				t.Errorf("T(%q) = %q, want %q", tt.messageID, got, tt.want)
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
		{name: "English is set", setup: func(ctx context.Context) context.Context { return i18n.SetLocale(ctx, i18n.LangEn) }, want: i18n.LangEn},
		{name: "nothing is set falls back to the default", setup: func(ctx context.Context) context.Context { return ctx }, want: i18n.DefaultLang},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.setup(context.Background())
			if got := templates.Locale(ctx); got != tt.want {
				t.Errorf("Locale() = %q, want %q", got, tt.want)
			}
		})
	}
}
