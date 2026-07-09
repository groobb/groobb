package settings

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	settingspage "github.com/groobb/groobb/go/internal/templates/pages/settings"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /settings - renders the settings hub, a list of links to the individual
// settings screens (email change for now). It is registered behind RequireAuth. The
// page is per-user and behind authentication, so it is marked noindex to keep it out
// of search indexes. The hub has no form and no per-user data, so it takes no page
// data.
//
// [Ja] Show GET /settings - 設定ハブを描画します。各設定画面 (今はメールアドレス変更)
// へのリンクの一覧です。RequireAuth の背後に登録されます。このページはユーザー固有かつ
// 認証の背後にあるため、検索インデックスから除外するよう noindex を付けます。ハブには
// フォームもユーザー固有のデータも無いため、ページデータを取りません。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_show_title")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, settingspage.Show()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "設定ページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
