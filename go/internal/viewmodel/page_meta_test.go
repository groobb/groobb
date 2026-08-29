package viewmodel_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
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
		locale          model.Locale
		wantTitle       string
		wantDescription string
	}{
		{
			name:            "Japanese",
			locale:          model.LocaleJa,
			wantTitle:       "Groobb",
			wantDescription: "Groobb は掲示板サービスです。",
		},
		{
			name:            "English",
			locale:          model.LocaleEn,
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

// TestDefaultPageMeta_SiteName verifies that the baseline metadata picks the
// site name up from the context, and leaves it empty when the context carries
// none, so a handler sets the name of its own page and nothing else of the
// title.
//
// [Ja] TestDefaultPageMeta_SiteName は、基準となるメタ情報がサイトの名前を context から
// 取得し、context がそれを持たないときは空のままにすることを検証します。これにより
// ハンドラーは自身のページの名前だけを設定し、タイトルの他の部分には触れません。
func TestDefaultPageMeta_SiteName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setSite  bool
		wantSite string
	}{
		{name: "コミュニティを持つインスタンス", setSite: true, wantSite: "ジャズ喫茶"},
		{name: "まだ立ち上げられていないインスタンス", setSite: false, wantSite: ""},
	}

	cfg := &config.Config{Env: "prod", AssetVersion: "abc123"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
			if tt.setSite {
				ctx = viewmodel.SetSiteName(ctx, tt.wantSite)
			}

			if got := viewmodel.DefaultPageMeta(ctx, cfg).SiteName; got != tt.wantSite {
				t.Errorf("SiteName = %q, want %q", got, tt.wantSite)
			}
		})
	}
}

// TestPageMeta_DocumentTitle verifies what the <title> element carries: the name
// of the page followed by the name of the site, in either locale, and the name
// of the page alone on an instance that has no community to name.
//
// [Ja] TestPageMeta_DocumentTitle は <title> 要素が運ぶものを検証します。どちらの
// ロケールでもページの名前に続いてサイトの名前が並び、名指すコミュニティを持たない
// インスタンスではページの名前だけになります。
func TestPageMeta_DocumentTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale model.Locale
		meta   viewmodel.PageMeta
		want   string
	}{
		{
			name:   "Japanese",
			locale: model.LocaleJa,
			meta:   viewmodel.PageMeta{Title: "ジャズ・ファンク", SiteName: "ジャズ喫茶"},
			want:   "ジャズ・ファンク - ジャズ喫茶",
		},
		{
			name:   "English",
			locale: model.LocaleEn,
			meta:   viewmodel.PageMeta{Title: "Sign in", SiteName: "Jazz Cafe"},
			want:   "Sign in - Jazz Cafe",
		},
		{
			name:   "コミュニティを持たないインスタンスはページの名前だけを運ぶ",
			locale: model.LocaleJa,
			meta:   viewmodel.PageMeta{Title: "ジャズ・ファンク"},
			want:   "ジャズ・ファンク",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			if got := tt.meta.DocumentTitle(ctx); got != tt.want {
				t.Errorf("DocumentTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
