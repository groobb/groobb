package viewmodel_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestDefaultPageMeta verifies that DefaultPageMeta sources the title and
// description from the locale stored in the context and carries the asset
// version through from the config.
//
// [Ja] TestDefaultPageMeta は DefaultPageMeta がタイトルと説明を context の
// ロケールから取得し、アセットバージョンを config から引き継ぐことを検証します。
func TestDefaultPageMeta(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		locale          string
		wantTitle       string
		wantDescription string
	}{
		{
			name:            "Japanese",
			locale:          i18n.LangJa,
			wantTitle:       "Groobb",
			wantDescription: "Groobb は掲示板サービスです。",
		},
		{
			name:            "English",
			locale:          i18n.LangEn,
			wantTitle:       "Groobb",
			wantDescription: "Groobb is a bulletin board service.",
		},
	}

	// Env is "prod" so GetAssetVersion returns the fixed value below instead of a
	// volatile dev timestamp, letting the test assert it deterministically.
	//
	// [Ja] Env を "prod" にすることで GetAssetVersion が揺れる dev のタイムスタンプ
	// ではなく下記の固定値を返し、テストで決定的に検証できるようにする。
	cfg := &config.Config{Env: "prod", AssetVersion: "abc123"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			meta := viewmodel.DefaultPageMeta(ctx, cfg)

			if meta.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", meta.Title, tt.wantTitle)
			}
			if meta.Description != tt.wantDescription {
				t.Errorf("Description = %q, want %q", meta.Description, tt.wantDescription)
			}
			if meta.AssetVersion != "abc123" {
				t.Errorf("AssetVersion = %q, want %q", meta.AssetVersion, "abc123")
			}
		})
	}
}
