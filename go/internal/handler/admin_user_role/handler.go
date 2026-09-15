// admin_user_roleパッケージは、利用者へロールを渡し、また取り上げるハンドラー
// (POST /admin/users/{id}/rolesとDELETE /admin/users/{id}/roles/{name}) を提供します。
// 管理画面の利用者一覧のボタンが送信する先であり、画面を通じてロールが渡る唯一の場所です。
package admin_user_role

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerは利用者のロールの付与と剥奪のHTTPハンドラーです。共通のエラー
// Rendererを保持するのは、どちらのルートも2種類の拒否に一覧ではなくページで応答する
// ためです。ロールを配ってはならないサインイン済みの訪問者と、存在しないアカウントや
// ロールを名指すアドレスです。フラッシュManagerは、何が起きたのかをリダイレクトの先へ
// 運びます。届いた送信への答えは、読み直された一覧であるためです。
type Handler struct {
	errorRenderer    *httperror.Renderer
	flashMgr         *session.FlashManager
	grantUserRoleUC  *usecase.GrantUserRoleUsecase
	revokeUserRoleUC *usecase.RevokeUserRoleUsecase
}

// NewHandlerは新しいadmin_user_role Handlerを作成します。
func NewHandler(
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	grantUserRoleUC *usecase.GrantUserRoleUsecase,
	revokeUserRoleUC *usecase.RevokeUserRoleUsecase,
) *Handler {
	return &Handler{
		errorRenderer:    errorRenderer,
		flashMgr:         flashMgr,
		grantUserRoleUC:  grantUserRoleUC,
		revokeUserRoleUC: revokeUserRoleUC,
	}
}

// refusedは、UseCaseが実行しなかった送信に応答し、拒否の理由を訪問者が受け取る
// 応答へ変えます。
//
// 権限による拒否と、存在しないアカウントやロールにはページで応答します。どちらも一覧が
// 何かを述べられるものではないためです。前者はこの訪問者が一覧を読んではならないことを、
// 後者は押された行が何も名指していないことを意味します。コミュニティが管理者のいない状態に
// なることには、一覧そのものと、フラッシュに載せた理由で応答します。その周りの行は、
// 訪問者にとって依然として次の手立てであるためです。
func (h *Handler) refused(w http.ResponseWriter, r *http.Request, err error, logMsg string) {
	ctx := r.Context()

	var ae *model.AppError
	if errors.As(err, &ae) {
		switch ae.Code {
		case model.AppErrCodeForbidden:
			slog.InfoContext(ctx, ae.LogString())
			h.errorRenderer.Forbidden(w, r)
			return
		case model.AppErrCodeResourceNotFound:
			slog.InfoContext(ctx, ae.LogString())
			h.errorRenderer.NotFound(w, r)
			return
		case model.AppErrCodeConflict:
			slog.InfoContext(ctx, ae.LogString())
			h.flashMgr.SetError(w, ae.UserMsg)
			http.Redirect(w, r, listingPath(r).String(), http.StatusSeeOther)
			return
		default:
			slog.ErrorContext(ctx, ae.LogString())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	slog.ErrorContext(ctx, logMsg, "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// listingPathは、送信が来た一覧のページであり、そこで応答します。行のフォームは、
// 一覧が読まれたときの絞り込みとページ番号を運ぶため、検索して見つけた相手に対する操作が、
// 訪問者を全員の最初のページへ送り返すことはありません。
//
// 2つの値は送信からのみ読みます。そしてこれらが、送信がアドレスについて決められる唯一の
// ものです。パスはここに書かれており、応答の行き先を送信が名指すことはできません。整数と
// して読めないページ番号や、最初のページより前の番号は、拒まずに最初のページとして読みます。
// 誤って書かれたものは何も無く、一覧はどちらにせよ応答できるページを持つためです。
func listingPath(r *http.Request) templates.Path {
	page, err := strconv.Atoi(r.PostFormValue(templates.PageParam))
	if err != nil || page < 1 {
		page = 1
	}

	return templates.AdminUsersPagePath(r.PostFormValue(templates.AdminUsersQueryParam), page)
}
