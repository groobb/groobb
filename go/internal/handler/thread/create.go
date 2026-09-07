package thread

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Create POST /b/{slug}/threads - starts a thread in the board the address
// names, with the submitted title, primary language and first post, and sends
// the author to the post they just wrote.
//
// The author is the account the session names. The form carries no field for it,
// and none would be read: a submission saying whose post it is could attribute
// one to somebody else. The route is registered behind RequireAuth, so the user
// in the context is there.
//
// The fields are read from the body alone. A form is submitted as a body, so a
// value appearing only in the query string was put there by something other than
// the form this route serves, and reading it would let a link write a post.
//
// A slug spelled differently from the stored one is not redirected the way the
// form's GET is. A redirect turns a POST into a GET, which would drop what was
// written; the board is resolved as the form's GET resolves it, the thread is
// started in it, and the canonical address arrives with the 303 that follows.
//
// [Ja] Create POST /b/{slug}/threads - アドレスが名指す掲示板に、送信されたタイトル・
// 主言語・最初の投稿でスレッドを立て、書き手を今書いた投稿へ送ります。
//
// 書き手はセッションが名指すアカウントです。フォームはそのためのフィールドを持たず、
// あっても読みません。誰の投稿かを述べる送信は、投稿を別人に帰属させうるためです。
// ルートは RequireAuth の背後に登録されるため、context のユーザーは存在します。
//
// フィールドはボディからのみ読みます。フォームはボディとして送信されるため、クエリ文字列
// にだけ現れる値は、このルートが配信するフォーム以外の何かが置いたものであり、それを読めば
// リンクが投稿を書けることになります。
//
// 保存された綴りと異なる slug は、フォームの GET のようにはリダイレクトしません。
// リダイレクトは POST を GET に変え、書かれたものを落としてしまいます。掲示板はフォームの
// GET が解決するのと同じように解決し、そこにスレッドを立て、正規のアドレスはその後に続く
// 303 で届きます。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

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
	form := newForm{
		Title:    r.PostFormValue("title"),
		Language: r.PostFormValue("language"),
		Body:     model.NormalizeLineBreaks(r.PostFormValue("body")),
	}

	output, err := h.createThreadUC.Execute(ctx, usecase.CreateThreadInput{
		BoardSlug: slug,
		UserID:    middleware.UserFromContext(ctx).ID,
		Title:     form.Title,
		Language:  form.Language,
		Body:      form.Body,
	})
	if err != nil {
		h.createRefused(w, r, slug, form, err)
		return
	}

	// The thread is answered at the post that started it rather than at its top,
	// so a browser lands on what was just written even once the thread holds more.
	//
	// [Ja] スレッドは、その先頭ではなく、それを始めた投稿の位置で応答する。スレッドが
	// もっと多くの投稿を持つようになっても、ブラウザが今書かれたものの位置に着地する
	// ようにするためである。
	target := templates.ThreadPath(viewmodel.ThreadID(output.ThreadID)) + templates.PostAnchor(output.Number)
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

// createRefused answers a submission that was not saved, turning what the
// UseCase refused it for into a status and a message.
//
// Everything a visitor can act on comes back as the form they submitted, holding
// what they wrote: a field they can correct, a wait they can sit out, or a
// failure they can try again. Only a board that is not there is answered with
// another page, since there is no form to draw for a board the address does not
// name.
//
// [Ja] createRefused は保存されなかった送信に応答し、UseCase が何を理由に拒否したかを
// ステータスとメッセージに変えます。
//
// 訪問者が手を打てるものは、いずれも送信したフォームとして、書いたものを保ったまま返って
// きます。直せるフィールド、待てば済む待ち時間、もう一度試せる失敗です。別のページで応答
// するのは、そこに無い掲示板だけです。アドレスがどの掲示板も名指していないとき、描くべき
// フォームが無いためです。
func (h *Handler) createRefused(w http.ResponseWriter, r *http.Request, slug string, form newForm, err error) {
	ctx := r.Context()

	if ve := model.AsValidationError(err); ve != nil {
		form.Errors = ve
		h.renderRefusedForm(w, r, http.StatusUnprocessableEntity, slug, form)
		return
	}

	ae := model.AsAppError(err)
	if ae == nil {
		slog.ErrorContext(ctx, "スレッドの作成に失敗", "error", err)
		form.Errors = formWideError(i18n.T(ctx, "validation_post_save_failed"))
		h.renderRefusedForm(w, r, http.StatusInternalServerError, slug, form)
		return
	}

	switch ae.Code {
	case model.AppErrCodeResourceNotFound:
		h.errorRenderer.NotFound(w, r)
	case model.AppErrCodeRateLimited:
		// The wait is stated to the browser as well as on the page, because what
		// refused the submission is the interval rather than anything about it.
		// It is already whole seconds, which is all the header admits.
		//
		// [Ja] 待ち時間はページだけでなくブラウザにも伝える。送信を拒否したのは、その
		// 送信の中身ではなく間隔だからである。値は既に整数秒であり、このヘッダーが
		// 受け付けるのもそれだけである。
		w.Header().Set("Retry-After", strconv.Itoa(int(ae.RetryAfter.Seconds())))
		form.Errors = formWideError(ae.Error())
		h.renderRefusedForm(w, r, http.StatusTooManyRequests, slug, form)
	case model.AppErrCodeForbidden:
		form.Errors = formWideError(ae.Error())
		h.renderRefusedForm(w, r, http.StatusForbidden, slug, form)
	default:
		slog.ErrorContext(ctx, ae.LogString())
		form.Errors = formWideError(i18n.T(ctx, "validation_post_save_failed"))
		h.renderRefusedForm(w, r, http.StatusInternalServerError, slug, form)
	}
}

// renderRefusedForm draws the creation form again at status, holding what was
// submitted. The board is resolved once more because the page names it and draws
// the trail it sits in, neither of which the submission carries.
//
// A board the address no longer names is answered with the 404 page, and a read
// that fails for any other reason with the plain error. Either way the
// submission is lost, which is the reason this read happens at all: without the
// board there is no page to put it back on.
//
// [Ja] renderRefusedForm は作成フォームを status で描き直し、送信されたものを保ちます。
// 掲示板をもう一度解決するのは、ページがそれを名指し、それが置かれた経路を描くためで、
// どちらも送信は運んでいないためです。
//
// アドレスがもう名指していない掲示板には 404 ページで応答し、それ以外の理由で読み取りが
// 失敗したときは素のエラーで応答します。どちらでも送信は失われますが、この読み取りを行う
// 理由自体がそこにあります。掲示板が無ければ、それを戻して置くページがありません。
func (h *Handler) renderRefusedForm(w http.ResponseWriter, r *http.Request, status int, slug string, form newForm) {
	ctx := r.Context()

	resolved, err := h.getBoardUC.Execute(ctx, usecase.GetBoardInput{Slug: slug})
	if err != nil {
		// Without the board there is no form to re-render, so return 404.
		// Validation may have rejected the input before the board was ever read.
		//
		// [Ja] 再描画に必要な掲示板が見つからないため、404 を返す。
		// 入力検証で拒否された場合は、掲示板がまだ読み取られていないこともある。
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "フォームの再描画のための掲示板の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.renderNew(w, r, status, resolved, form); err != nil {
		slog.ErrorContext(ctx, "スレッド作成ページの再描画に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
