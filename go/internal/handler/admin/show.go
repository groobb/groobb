package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	adminpage "github.com/groobb/groobb/go/internal/templates/pages/admin"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /admin - renders the admin hub, a list of links to the community's
// administration screens (the user list for now). It is registered behind
// RequireAuth, which settles that someone is signed in; whether that someone may
// open the page is settled by the UseCase, and a refusal is answered with the
// shared 403 page.
//
// The page is marked noindex because it is behind authentication and admitted to
// a few people, so there is nothing here for a searcher. It renders in the Default
// layout, as the settings hub does: the community shell's sidebar is for moving
// around the community, and this page is not part of it.
//
// [Ja] Show GET /admin - 管理ハブを描画します。コミュニティの管理画面 (今は利用者一覧)
// へのリンクの一覧です。RequireAuth の背後に登録され、そこで誰かがサインインしていることが
// 決まります。その誰かがこのページを開いてよいかどうかを決めるのは UseCase で、拒否には
// 共通の 403 ページで応答します。
//
// 認証の背後にあり、許されるのは数人であるため、検索する人に差し出すものはありません。
// そこで noindex を付けます。設定ハブと同じく Default レイアウトで描画します。コミュニティの
// シェルのサイドバーはコミュニティの中を移動するためのもので、このページはその一部では
// ないためです。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		slog.ErrorContext(ctx, "管理ハブにユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.getAdminHomeUC.Execute(ctx, usecase.GetAdminHomeInput{
		Actor: usecase.UserActor(user.ID),
	}); err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) && ae.Code == model.AppErrCodeForbidden {
			slog.InfoContext(ctx, ae.LogString())
			h.errorRenderer.Forbidden(w, r)
			return
		}
		slog.ErrorContext(ctx, "管理画面の権限の確認に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "admin_show_title")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, adminpage.Show()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "管理ハブのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
