// Package admin_user_role provides the handlers that hand a role to a user and
// take it back (POST /admin/users/{id}/roles and
// DELETE /admin/users/{id}/roles/{name}). They are what the buttons of the admin
// user listing submit to, and the only place a role changes hands through a
// screen.
//
// [Ja] admin_user_role パッケージは、利用者へロールを渡し、また取り上げるハンドラー
// (POST /admin/users/{id}/roles と DELETE /admin/users/{id}/roles/{name}) を提供します。
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

// Handler is the HTTP handler for granting and revoking a user's role. It holds
// the shared error renderer because both routes answer two refusals with a page
// rather than with the listing: a signed-in visitor who may not hand roles out,
// and an address naming an account or a role that is not there. The flash
// manager carries what happened across the redirect, since the answer to a
// submission that did land is the listing read again.
//
// [Ja] Handler は利用者のロールの付与と剥奪の HTTP ハンドラーです。共通のエラー
// Renderer を保持するのは、どちらのルートも 2 種類の拒否に一覧ではなくページで応答する
// ためです。ロールを配ってはならないサインイン済みの訪問者と、存在しないアカウントや
// ロールを名指すアドレスです。フラッシュ Manager は、何が起きたのかをリダイレクトの先へ
// 運びます。届いた送信への答えは、読み直された一覧であるためです。
type Handler struct {
	errorRenderer    *httperror.Renderer
	flashMgr         *session.FlashManager
	grantUserRoleUC  *usecase.GrantUserRoleUsecase
	revokeUserRoleUC *usecase.RevokeUserRoleUsecase
}

// NewHandler creates a new admin_user_role Handler.
//
// [Ja] NewHandler は新しい admin_user_role Handler を作成します。
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

// refused answers a submission the UseCase did not carry out, turning what it
// refused for into the response the visitor gets.
//
// A refusal of permission and an account or role that is not there are answered
// with a page, because neither is something the listing can say anything about:
// one means this visitor may not read it, and the other means the row that was
// pressed names nothing. The community being left without an administrator is
// answered with the listing itself and the reason in a flash, since the rows
// around it are still the visitor's next move.
//
// [Ja] refused は、UseCase が実行しなかった送信に応答し、拒否の理由を訪問者が受け取る
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

// listingPath is the page of the listing the submission came from, which is
// where it is answered. The row's form carries the search and the page number
// the listing was read at, so acting on someone found by searching does not send
// the visitor back to the first page of everyone.
//
// The two values are read from the submission alone, and they are the only thing
// about the address that a submission decides: the path is written here, so no
// submission can name where the response goes. A page number that is not a whole
// number, or is below the first page, is read as the first page rather than
// refused — nothing was written wrongly, and the listing has a page to answer at
// either way.
//
// [Ja] listingPath は、送信が来た一覧のページであり、そこで応答します。行のフォームは、
// 一覧が読まれたときの絞り込みとページ番号を運ぶため、検索して見つけた相手に対する操作が、
// 訪問者を全員の最初のページへ送り返すことはありません。
//
// 2 つの値は送信からのみ読みます。そしてこれらが、送信がアドレスについて決められる唯一の
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
