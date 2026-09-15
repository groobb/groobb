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

// Delete DELETE /admin/users/{id}/roles/{name} - アドレスが名指すロールを、同じく
// アドレスが名指すアカウントから取り上げます。ルートは一覧の剥奪フォームから
// _method=DELETEのオーバーライドで到達し、隣の付与と同じくRequireAuthの背後に登録されます。
//
// アカウントとロールのどちらもアドレスが名指します。取り除かれるのは1つの割当であり、
// この割当こそがこのアドレスの表すものであるためです。
//
// 自分自身の管理者ロールを外すことは許され、そのとき訪問者は一覧ではなくトップページへ
// 送られます。一覧は真っ先に読めなくなるものであり、自ら望んで送った要求に403ページで
// 応答すれば、それは失敗として読まれるためです。それ以外の剥奪は、押された一覧で応答します。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "ロールの剥奪にユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	targetUserID, ok := model.ParseUserID(chi.URLParam(r, "id"))
	if !ok {
		slog.InfoContext(ctx, "利用者idとして読めない値でロールの剥奪に到達", "id", chi.URLParam(r, "id"))
		h.errorRenderer.NotFound(w, r)
		return
	}

	revoked, err := h.revokeUserRoleUC.Execute(ctx, usecase.RevokeUserRoleInput{
		Actor:        usecase.UserActor(actor.ID),
		TargetUserID: targetUserID,
		RoleName:     model.RoleName(chi.URLParam(r, "name")),
	})
	if err != nil {
		h.refused(w, r, err, "ロールの剥奪に失敗")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_user_role_revoked", map[string]any{"Atname": revoked.TargetAtname}))

	target := listingPath(r)
	if targetUserID == actor.ID {
		target = templates.HomePath()
	}
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}
