package viewmodel_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestDefaultPageMetaはDefaultPageMetaがタイトルと説明をcontextの
// ロケールから取得し、アセットバージョンをconfigから引き継ぐことを検証します。
func TestDefaultPageMeta(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		locale          model.Locale
		wantTitle       string
		wantDescription string
	}{
		{
			name:            "日本語",
			locale:          model.LocaleJa,
			wantTitle:       "Groobb",
			wantDescription: "Groobbは掲示板サービスです。",
		},
		{
			name:            "英語",
			locale:          model.LocaleEn,
			wantTitle:       "Groobb",
			wantDescription: "Groobb is a bulletin board service.",
		},
	}

	// Envを "prod" にすることでGetAssetVersionが揺れるdevのタイムスタンプ
	// ではなく下記の固定値を返し、テストで決定的に検証できるようにする。
	cfg := &config.Config{Env: "prod", AssetVersion: "abc123"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			meta := viewmodel.DefaultPageMeta(ctx, cfg)

			if meta.Title != tt.wantTitle {
				t.Errorf("Title = %q、期待値 = %q", meta.Title, tt.wantTitle)
			}
			if meta.Description != tt.wantDescription {
				t.Errorf("Description = %q、期待値 = %q", meta.Description, tt.wantDescription)
			}
			if meta.AssetVersion != "abc123" {
				t.Errorf("AssetVersion = %q、期待値 = %q", meta.AssetVersion, "abc123")
			}
		})
	}
}

// TestDefaultPageMeta_SiteNameは、基準となるメタ情報がサイトの名前をcontextから
// 取得し、contextがそれを持たないときは空のままにすることを検証します。これにより
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
				t.Errorf("SiteName = %q、期待値 = %q", got, tt.wantSite)
			}
		})
	}
}

// TestPageMeta_DocumentTitleは <title> 要素が運ぶものを検証します。どちらの
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
			name:   "日本語",
			locale: model.LocaleJa,
			meta:   viewmodel.PageMeta{Title: "ジャズ・ファンク", SiteName: "ジャズ喫茶"},
			want:   "ジャズ・ファンク - ジャズ喫茶",
		},
		{
			name:   "英語",
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
				t.Errorf("DocumentTitle() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}
