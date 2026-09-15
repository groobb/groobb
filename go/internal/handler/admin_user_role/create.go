package admin_user_role

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /admin/users/{id}/roles - アドレスが名指すアカウントへ、送信が名指す
// ロールを与え、それが押された一覧で応答します。
//
// ロールを渡す側はセッションが名指すアカウントです。送信はそのためのフィールドを持たず、
// あっても読みません。誰が付与するのかを述べるフォームは、誰もが管理者として付与できる
// ことを意味するためです。ルートはRequireAuthの背後に登録されるためcontextのユーザーは
// 存在し、そのユーザーがロールを配ってよいかどうかはUseCaseが決めます。
//
// ロールはリクエストのボディからのみ読みます。このアドレスが名指すコレクションは、どの
// ロールを加えてもそのアカウントのロールであるため、加えるロールはアドレスではなく送信された
// ものが名指します。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "ロールの付与にユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	targetUserID, ok := model.ParseUserID(chi.URLParam(r, "id"))
	if !ok {
		slog.InfoContext(ctx, "利用者idとして読めない値でロールの付与に到達", "id", chi.URLParam(r, "id"))
		h.errorRenderer.NotFound(w, r)
		return
	}

	granted, err := h.grantUserRoleUC.Execute(ctx, usecase.GrantUserRoleInput{
		Actor:        usecase.UserActor(actor.ID),
		TargetUserID: targetUserID,
		RoleName:     model.RoleName(r.PostFormValue(templates.AdminUserRoleNameParam)),
	})
	if err != nil {
		h.refused(w, r, err, "ロールの付与に失敗")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_user_role_granted", map[string]any{"Atname": granted.TargetAtname}))
	http.Redirect(w, r, listingPath(r).String(), http.StatusSeeOther)
}
