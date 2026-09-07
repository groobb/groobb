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

// Create POST /t/{id}/posts - adds the submitted body to the thread the address
// names, and sends its author to the post they just wrote.
//
// The author is the account the session names. The form carries no field for it,
// and none would be read: a submission saying whose post it is could attribute
// one to somebody else. The route is registered behind RequireAuth, so the user
// in the context is there.
//
// The body is read from the request body alone. A form is submitted as a body,
// so a value appearing only in the query string was put there by something other
// than the form this route serves, and reading it would let a link write a post.
//
// The id is checked before anything is read, as the thread's page checks it: a
// path that is not the decimal spelling of a thread's id names no thread. Unlike
// that page, a spelling of the same id it would not have written is not
// redirected to the canonical one — a redirect turns a POST into a GET, which
// would drop what was written. The reply is saved to the thread the id resolves
// to, and the canonical address arrives with the 303 that follows.
//
// [Ja] Create POST /t/{id}/posts - 送信された本文を、アドレスが名指すスレッドへ加え、
// その書き手を今書いた投稿へ送ります。
//
// 書き手はセッションが名指すアカウントです。フォームはそのためのフィールドを持たず、
// あっても読みません。誰の投稿かを述べる送信は、投稿を別人に帰属させうるためです。
// ルートは RequireAuth の背後に登録されるため、context のユーザーは存在します。
//
// 本文はリクエストのボディからのみ読みます。フォームはボディとして送信されるため、
// クエリ文字列にだけ現れる値は、このルートが配信するフォーム以外の何かが置いたものであり、
// それを読めばリンクが投稿を書けることになります。
//
// id はスレッドのページがそうするように、何かを読む前に検査します。スレッドの id の
// 10 進表記でないパスはどのスレッドも名指しません。そのページと違い、同じ id をページが
// 書かない綴りで表すものは正規の形へリダイレクトしません。リダイレクトは POST を GET に
// 変え、書かれたものを落としてしまいます。返信は id が解決したスレッドへ保存し、正規の
// アドレスはその後に続く 303 で届きます。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := model.ParseThreadID(chi.URLParam(r, "id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	// The body's line endings are settled here, where the submission is read, so
	// that the value saved and the value drawn back both come out of one
	// normalization. Normalizing again for the re-render would leave two places
	// to keep in step, and a rule that grew on one side would quietly show the
	// visitor something other than what would have been stored.
	//
	// [Ja] 本文の改行は、送信を読むこの場所で確定させる。保存される値と描き戻される値の
	// どちらも 1 回の正規化から出るようにするためである。再描画のために改めて正規化すると
	// 歩調を合わせ続ける場所が 2 つになり、片方だけで育った規則は、保存されるはずだった
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

	// The thread is answered at the post that was just written rather than at its
	// top, which is where a thread of a thousand posts would otherwise land the
	// visitor who added the last one.
	//
	// [Ja] スレッドは、その先頭ではなく今書かれた投稿の位置で応答する。そうしなければ、
	// 1000 件の投稿を持つスレッドは、その最後の 1 件を加えた訪問者を先頭に着地させる
	// ことになる。
	target := templates.ThreadPath(viewmodel.ThreadID(id)) + templates.PostAnchor(output.Number)
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

// refusedReply is a submission that was not saved, as the page it comes back on
// carries it: what was written, and why it was refused.
//
// A lock and a message never travel together. A thread that takes no post is not
// answered by correcting a field or by waiting, so that page states the reason
// and keeps the text; every other refusal comes back as the form the reply was
// written in, holding the message about it.
//
// [Ja] refusedReply は保存されなかった送信を、それが戻ってくるページが運ぶ形で表します。
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

// createRefused answers a submission that was not saved, turning what the
// UseCase refused it for into a status and what the page says.
//
// Everything a visitor can act on comes back holding what they wrote: a field
// they can correct, a wait they can sit out, or a failure they can try again. A
// thread that takes no further post comes back holding it too, so it can be
// copied out, but without a way to send it again — a button that would be
// refused a second time is not a way out. Only a thread that is not there is
// answered with another page, since there is no reply to draw for a thread the
// address does not name.
//
// [Ja] createRefused は保存されなかった送信に応答し、UseCase が何を理由に拒否したかを
// ステータスとページの述べることに変えます。
//
// 訪問者が手を打てるものは、いずれも書いたものを保ったまま返ってきます。直せるフィールド、
// 待てば済む待ち時間、もう一度試せる失敗です。これ以上の投稿を受け付けないスレッドも、
// 写し取れるようにそれを保ったまま返ってきますが、もう一度送る手立ては伴いません。
// 2 度目も拒否されるボタンは出口ではないためです。別のページで応答するのは、そこに無い
// スレッドだけです。アドレスがどのスレッドも名指していないとき、描くべき返信がありません。
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
	case model.AppErrCodeThreadLocked:
		// The reasons travel to the page as they were decided, and the notice
		// drawn from them is the whole of what is said. A message about the
		// submission would stand beside them saying the same thing in weaker
		// words.
		//
		// [Ja] 理由は決められたままの形でページへ渡り、そこから描かれる案内が述べることの
		// すべてになる。送信についてのメッセージを添えれば、同じことをより弱い言葉で
		// 述べるものがその隣に並ぶ。
		h.renderRefused(w, r, http.StatusConflict, id, refusedReply{Body: body, Lock: viewmodel.NewThreadLock(ae.LockReasons)})
	case model.AppErrCodeRateLimited:
		// The wait is stated to the browser as well as on the page, because what
		// refused the submission is the interval rather than anything about it.
		// It is already whole seconds, which is all the header admits.
		//
		// [Ja] 待ち時間はページだけでなくブラウザにも伝える。送信を拒否したのは、その
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

// renderRefused draws the page a refused reply comes back on at status. The
// thread is read again because the page names it and links to it, neither of
// which the submission carries, and its posts are left unread: the page shows
// the reply rather than the conversation.
//
// A thread the address no longer names is answered with the 404 page, and a read
// that fails for any other reason with the plain error. Either way the
// submission is lost, which is the reason this read happens at all: without the
// thread there is no page to put it back on.
//
// The page carries noindex and is sent no-store. It answers a submission rather
// than an address a search result would show, and it holds what the visitor
// wrote.
//
// [Ja] renderRefused は、拒否された返信が戻ってくるページを status で描きます。スレッドを
// もう一度読むのは、ページがそれを名指してリンクするためで、どちらも送信は運んでいない
// ためです。その投稿は読みません。このページが見せるのは会話ではなく返信だからです。
//
// アドレスがもう名指していないスレッドには 404 ページで応答し、それ以外の理由で読み取りが
// 失敗したときは素のエラーで応答します。どちらでも送信は失われますが、この読み取りを行う
// 理由自体がそこにあります。スレッドが無ければ、それを戻して置くページがありません。
//
// このページは noindex を持ち、no-store で送ります。検索結果が見せるアドレスではなく送信に
// 応答するページであり、訪問者が書いたものを持つためです。
func (h *Handler) renderRefused(w http.ResponseWriter, r *http.Request, status int, id model.ThreadID, reply refusedReply) {
	ctx := r.Context()

	resolved, err := h.getThreadSummaryUC.Execute(ctx, usecase.GetThreadSummaryInput{ID: id})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "返信の再試行ページのためのスレッドの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	nav, err := h.getCommunityNavigationUC.Execute(ctx)
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

	// The sidebar's sign-in links point back at the thread rather than at the
	// address of the request being answered. This handler answers under the
	// address the reply is posted to, which accepts nothing but a submission: a
	// visitor sent to sign-in and returned there would arrive at a POST-only URL
	// with a GET.
	//
	// [Ja] サイドバーのサインインのリンクは、応答中のリクエストのアドレスではなく
	// スレッドを指す。このハンドラーが応答するのは返信の送信先のアドレスであり、そこは
	// 送信しか受け付けない。サインインへ送られてそこへ戻された訪問者は、POST しか
	// 受け付けない URL に GET で辿り着くことになる。
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

// formWideError carries message as an error about the submission as a whole,
// which is where a refusal that is not about a field belongs: no input of the
// visitor's is at fault, so none of them is marked as such.
//
// [Ja] formWideError は message を、送信そのものについてのエラーとして運びます。
// フィールドについてではない拒否が属する場所です。訪問者のどの入力にも落ち度が無いため、
// そのいずれにも印を付けません。
func formWideError(message string) *model.ValidationError {
	errors := model.NewValidationError()
	errors.AddGlobal(message)
	return errors
}
