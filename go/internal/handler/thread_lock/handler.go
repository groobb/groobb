// thread_lockパッケージは、モデレーターがスレッドに掛けるロックのハンドラーを提供
// します。確認ページ (GET /t/{id}/lock/new)、ロックを掛けること (POST /t/{id}/lock)、
// そしてそれを外すこと (DELETE /t/{id}/lock) です。3つともRequireAuthの背後にあり、
// サインイン済みの訪問者がそのどれを行ってよいかは、それぞれが呼ぶUseCaseが決めます。
//
// 掛けることと外すことが1つのアドレスを共有するのは、どちらもスレッドが持つ1つのロックに
// 対して行われるためです。外すことに専用の確認ページはありません。それはスレッドを元の姿に
// 戻すことであり、事前に量るべきものも、履歴が保つ注記もないためです。
package thread_lock

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// HandlerはスレッドのロックのHTTPハンドラーです。共通のエラーRendererを保持するのは、
// 3つのルートがいずれも同じ3種類の拒否に、スレッドではなくページで応答するためです。
// モデレーションを許されていない訪問者、どのスレッドも名指していないアドレス、そして
// コミュニティが視界の外へ移したスレッドです。フラッシュManagerは、何が起きたのかを
// リダイレクトの先へ運びます。届いた操作への答えは、読み直されたスレッドであるためです。
//
// 確認ページが、スレッドのページが使うUseCaseではなく専用のUseCaseで読むのは、それが必要と
// するのがスレッドとそれに働きかける権限であって、その下の会話ではないためです。
type Handler struct {
	cfg             *config.Config
	errorRenderer   *httperror.Renderer
	flashMgr        *session.FlashManager
	getModerationUC *usecase.GetThreadModerationUsecase
	lockThreadUC    *usecase.LockThreadUsecase
	unlockThreadUC  *usecase.UnlockThreadUsecase
}

// NewHandlerは新しいthread_lock Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getModerationUC *usecase.GetThreadModerationUsecase,
	lockThreadUC *usecase.LockThreadUsecase,
	unlockThreadUC *usecase.UnlockThreadUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		errorRenderer:   errorRenderer,
		flashMgr:        flashMgr,
		getModerationUC: getModerationUC,
		lockThreadUC:    lockThreadUC,
		unlockThreadUC:  unlockThreadUC,
	}
}

// refusedは、UseCaseが実行しなかった要求に応答し、拒否の理由を訪問者が受け取る応答へ
// 変えます。
//
// 既知の3つの拒否にはページで応答します。どれもスレッドのロックの画面が何かを述べられる
// ものではないためです。訪問者がここに居てはならないか、アドレスがどのスレッドも名指して
// いないか、スレッドがもう示されていないかです。応答したかどうかを返すのは、自身で扱う拒否を
// 持つ呼び出し元 (長すぎる注記。これはフォームに載って戻ってきます) が、まずこれに尋ね、
// ここで答えが決まらなかったときにフォームを描けるようにするためです。
func (h *Handler) refused(w http.ResponseWriter, r *http.Request, err error) bool {
	ctx := r.Context()

	ae := model.AsAppError(err)
	if ae == nil {
		return false
	}

	switch ae.Code {
	case model.AppErrCodeForbidden:
		slog.InfoContext(ctx, ae.LogString())
		h.errorRenderer.Forbidden(w, r)
	case model.AppErrCodeResourceNotFound:
		slog.InfoContext(ctx, ae.LogString())
		h.errorRenderer.NotFound(w, r)
	case model.AppErrCodeResourceUnpublished:
		slog.InfoContext(ctx, ae.LogString())
		h.errorRenderer.Unpublished(w, r)
	default:
		slog.ErrorContext(ctx, ae.LogString())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
	return true
}
