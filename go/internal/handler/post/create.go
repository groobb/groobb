package post

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	postpage "github.com/groobb/groobb/go/internal/templates/pages/post"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Create POST /t/{id}/posts - 送信された本文を、アドレスが名指すスレッドへ加え、
// その書き手を今書いた投稿へ送ります。
//
// 書き手はセッションが名指すアカウントです。フォームはそのためのフィールドを持たず、
// あっても読みません。誰の投稿かを述べる送信は、投稿を別人に帰属させうるためです。
// ルートはRequireAuthの背後に登録されるため、contextのユーザーは存在します。
//
// 本文はリクエストのボディからのみ読みます。フォームはボディとして送信されるため、
// クエリ文字列にだけ現れる値は、このルートが配信するフォーム以外の何かが置いたものであり、
// それを読めばリンクが投稿を書けることになります。
//
// idはスレッドのページがそうするように、何かを読む前に検査します。スレッドのidの
// 10進表記でないパスはどのスレッドも名指しません。そのページと違い、同じidをページが
// 書かない綴りで表すものは正規の形へリダイレクトしません。リダイレクトはPOSTをGETに
// 変え、書かれたものを落としてしまいます。返信はidが解決したスレッドへ保存し、正規の
// アドレスはその後に続く303で届きます。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := model.ParseThreadID(chi.URLParam(r, "id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	// 本文の改行は、送信を読むこの場所で確定させる。保存される値と描き戻される値の
	// どちらも1回の正規化から出るようにするためである。再描画のために改めて正規化すると
	// 歩調を合わせ続ける場所が2つになり、片方だけで育った規則は、保存されるはずだった
	// ものとは別のものを訪問者に黙って見せることになる。
	body := model.NormalizeLineBreaks(r.PostFormValue("body"))

	output, err := h.createPostUC.Execute(ctx, usecase.CreatePostInput{
		ThreadID: id,
		UserID:   middleware.UserFromContext(ctx).ID,
		Body:     body,
	})
	if err != nil {
		h.createRefused(w, r, id, body, err)
		return
	}

	// スレッドは、その先頭ではなく今書かれた投稿の位置で応答する。そうしなければ、
	// 1000件の投稿を持つスレッドは、その最後の1件を加えた訪問者を先頭に着地させる
	// ことになる。
	target := templates.ThreadPostAnchorPath(viewmodel.ThreadID(id), output.Number)
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

// refusedReplyは保存されなかった送信を、それが戻ってくるページが運ぶ形で表します。
// 書かれたものと、拒否された理由です。
//
// ロックとメッセージが同時に伴うことはありません。投稿を受け付けないスレッドは、
// フィールドを直しても待っても応じないため、そのページは理由を述べてテキストを保ちます。
// それ以外の拒否は、返信が書かれたフォームとして、それについてのメッセージを載せて
// 戻ってきます。
type refusedReply struct {
	Body   string
	Lock   viewmodel.ThreadLock
	Errors *model.ValidationError
}

// createRefusedは保存されなかった送信に応答し、UseCaseが何を理由に拒否したかを
// ステータスとページの述べることに変えます。
//
// 訪問者が手を打てるものは、いずれも書いたものを保ったまま返ってきます。直せるフィールド、
// 待てば済む待ち時間、もう一度試せる失敗です。これ以上の投稿を受け付けないスレッドも、
// 写し取れるようにそれを保ったまま返ってきますが、もう一度送る手立ては伴いません。
// 2度目も拒否されるボタンは出口ではないためです。別のページで応答するのは、描くべき返信が
// 無い場合だけです。アドレスがどのスレッドも名指していない場合と、コミュニティがそのスレッドを
// 見えない場所へ移した場合です。
func (h *Handler) createRefused(w http.ResponseWriter, r *http.Request, id model.ThreadID, body string, err error) {
	ctx := r.Context()

	if ve := model.AsValidationError(err); ve != nil {
		h.renderRefused(w, r, http.StatusUnprocessableEntity, id, refusedReply{Body: body, Errors: ve})
		return
	}

	ae := model.AsAppError(err)
	if ae == nil {
		slog.ErrorContext(ctx, "返信の作成に失敗", "error", err)
		h.renderRefused(w, r, http.StatusInternalServerError, id, refusedReply{Body: body, Errors: formWideError(i18n.T(ctx, "validation_post_save_failed"))})
		return
	}

	switch ae.Code {
	case model.AppErrCodeResourceNotFound:
		h.errorRenderer.NotFound(w, r)
	case model.AppErrCodeResourceUnpublished:
		// フォームが開かれてから送信が届くまでの間に、スレッドが見えない場所へ移された。
		// 返信を戻して置くページは無く、訪問者にはスレッドに何が起きたかを伝える。コミュニティが
		// もう示していないスレッドのフォームを渡すのではない。
		h.errorRenderer.Unpublished(w, r)
	case model.AppErrCodeThreadLocked:
		// 理由は決められたままの形でページへ渡り、そこから描かれる案内が述べることの
		// すべてになる。送信についてのメッセージを添えれば、同じことをより弱い言葉で
		// 述べるものがその隣に並ぶ。
		h.renderRefused(w, r, http.StatusConflict, id, refusedReply{Body: body, Lock: viewmodel.NewThreadLock(ae.LockReasons)})
	case model.AppErrCodeRateLimited:
		// 待ち時間はページだけでなくブラウザにも伝える。送信を拒否したのは、その
		// 送信の中身ではなく間隔だからである。値は既に整数秒であり、このヘッダーが
		// 受け付けるのもそれだけである。
		w.Header().Set("Retry-After", strconv.Itoa(int(ae.RetryAfter.Seconds())))
		h.renderRefused(w, r, http.StatusTooManyRequests, id, refusedReply{Body: body, Errors: formWideError(ae.Error())})
	case model.AppErrCodeForbidden:
		h.renderRefused(w, r, http.StatusForbidden, id, refusedReply{Body: body, Errors: formWideError(ae.Error())})
	default:
		slog.ErrorContext(ctx, ae.LogString())
		h.renderRefused(w, r, http.StatusInternalServerError, id, refusedReply{Body: body, Errors: formWideError(i18n.T(ctx, "validation_post_save_failed"))})
	}
}

// renderRefusedは、拒否された返信が戻ってくるページをstatusで描きます。スレッドを
// もう一度読むのは、ページがそれを名指してリンクするためで、どちらも送信は運んでいない
// ためです。その投稿は読みません。このページが見せるのは会話ではなく返信だからです。
//
// アドレスがもう名指していないスレッドには404ページで応答し、非公開にされたスレッドには
// その旨を述べるページで応答し、それ以外の理由で読み取りが失敗したときは素のエラーで応答します。どちらでも送信は失われますが、この読み取りを行う
// 理由自体がそこにあります。スレッドが無ければ、それを戻して置くページがありません。
//
// このページはnoindexを持ち、no-storeで送ります。検索結果が見せるアドレスではなく送信に
// 応答するページであり、訪問者が書いたものを持つためです。
func (h *Handler) renderRefused(w http.ResponseWriter, r *http.Request, status int, id model.ThreadID, reply refusedReply) {
	ctx := r.Context()

	resolved, err := h.getThreadSummaryUC.Execute(ctx, usecase.GetThreadSummaryInput{ID: id})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil {
			switch ae.Code {
			case model.AppErrCodeResourceNotFound:
				h.errorRenderer.NotFound(w, r)
				return
			case model.AppErrCodeResourceUnpublished:
				h.errorRenderer.Unpublished(w, r)
				return
			}
		}
		slog.ErrorContext(ctx, "返信の再試行ページのためのスレッドの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	nav, err := h.getCommunityNavigationUC.Execute(ctx, usecase.GetCommunityNavigationInput{
		UserID: middleware.UserIDFromContext(ctx),
	})
	if err != nil {
		slog.ErrorContext(ctx, "コミュニティのナビゲーションの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	thread := resolved.Thread
	threadID := viewmodel.ThreadID(thread.ID)

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "post_create_title")
	meta.NoIndex = true

	// サイドバーのサインインのリンクは、応答中のリクエストのアドレスではなく
	// スレッドを指す。このハンドラーが応答するのは返信の送信先のアドレスであり、そこは
	// 送信しか受け付けない。サインインへ送られてそこへ戻された訪問者は、POSTしか
	// 受け付けないURLにGETで辿り着くことになる。
	returnTo := middleware.SanitizeReturnTo(templates.ThreadPath(threadID).String())
	sidebar := viewmodel.NewSidebar(nav, middleware.UserFromContext(ctx), middleware.CSRFTokenFromContext(ctx), returnTo)

	pageData := postpage.CreatePageData{
		ThreadID:       threadID,
		ThreadTitle:    thread.Title,
		ThreadLanguage: viewmodel.NewThreadLanguage(thread.Language),
		Lock:           reply.Lock,
		PostLimit:      model.ThreadPostLimit,
		Reply: components.PostFormData{
			CSRFToken:           middleware.CSRFTokenFromContext(ctx),
			Action:              templates.ThreadPostsPath(threadID),
			Body:                reply.Body,
			PostIntervalSeconds: int(model.PostInterval.Seconds()),
			Errors:              reply.Errors,
		},
	}
	columns := layouts.CommunityColumns{
		Center:         postpage.Create(pageData),
		MainLabelledBy: postpage.CreateHeadingID,
		Main:           layouts.CommunityCenterColumn,
	}
	layoutData := layouts.CommunityLayoutData{Meta: meta, Sidebar: sidebar, Columns: columns}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	if err := layouts.Community(layoutData).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "返信の再試行ページのレンダリングに失敗", "error", err)
	}
}

// formWideErrorはmessageを、送信そのものについてのエラーとして運びます。
// フィールドについてではない拒否が属する場所です。訪問者のどの入力にも落ち度が無いため、
// そのいずれにも印を付けません。
func formWideError(message string) *model.ValidationError {
	errors := model.NewValidationError()
	errors.AddGlobal(message)
	return errors
}
