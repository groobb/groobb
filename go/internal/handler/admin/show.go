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

// Show GET /admin - 管理ハブを描画します。コミュニティの管理画面 (今は利用者一覧)
// へのリンクの一覧です。RequireAuthの背後に登録され、そこで誰かがサインインしていることが
// 決まります。その誰かがこのページを開いてよいかどうかを決めるのはUseCaseで、拒否には
// 共通の403ページで応答します。
//
// 認証の背後にあり、許されるのは数人であるため、検索する人に差し出すものはありません。
// そこでnoindexを付けます。設定ハブと同じくDefaultレイアウトで描画します。コミュニティの
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
