package sign_up

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	signuppage "github.com/groobb/groobb/go/internal/templates/pages/sign_up"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /sign_up - accepts an email, issues an email confirmation, and
// redirects to the code-entry step. On a validation error the form is
// re-rendered with the messages (422); the CSRF check is enforced upstream by
// the CSRF middleware, so it is not repeated here.
//
// [Ja] Create POST /sign_up - email を受け付け、メール確認を発行し、コード入力ステップ
// へリダイレクトします。バリデーションエラー時はメッセージ付きでフォームを再描画します
// (422)。CSRF 検証は上流の CSRF ミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")

	// Verify the Turnstile token before the UseCase, as a request-level bot gate:
	// a non-pass (an unsolved / bot-forged widget) and a siteverify failure both
	// stop the request here so it never reaches the UseCase (no confirmation mail
	// is enqueued). Both are logged at warn — Groobb has no error tracker, so warn
	// is the ceiling — and surfaced to the user as a form-wide message, 422. The
	// disabled dev / test setup (empty secret key) passes verification, so this
	// gate is transparent there.
	//
	// [Ja] UseCase の前に Turnstile トークンを検証する。リクエストレベルの Bot ゲートとして、
	// 非通過 (未解決 / Bot による偽造ウィジェット) と siteverify の失敗のどちらもここで
	// リクエストを止め、UseCase へは到達させない (確認メールは投入されない)。どちらも warn で
	// ログする (Groobb はエラートラッカー未連携のため warn が上限)。ユーザーにはフォーム全体の
	// メッセージとして 422 で表示する。無効化された dev / test 構成 (シークレットキー空) は検証を
	// 通過するため、そこではこのゲートは透過的に働く。
	if passed, err := h.turnstile.Verify(ctx, r.FormValue("cf-turnstile-response")); err != nil || !passed {
		// A plain non-pass (passed == false, err == nil) is an expected bot
		// rejection, not a system error, so attach the error attribute only when
		// there actually is one — otherwise the log carries an empty error=<nil>.
		//
		// [Ja] 単なる非通過 (passed == false, err == nil) は想定内の Bot 拒否であり
		// システムエラーではないため、error 属性は実際にエラーがあるときだけ付ける。
		// そうしないとログに空の error=<nil> が残る。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstile 検証を通過しなかったためサインアップを受け付けない", attrs...)
		formErrors := model.NewValidationError()
		formErrors.AddGlobal(i18n.T(ctx, "validation_turnstile_failed"))
		h.renderNew(w, r, http.StatusUnprocessableEntity, signuppage.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Email:      email,
			FormErrors: formErrors,
		})
		return
	}

	output, err := h.createSignUpUC.Execute(ctx, usecase.CreateSignUpInput{
		Email:  email,
		Locale: i18n.GetLocale(ctx),
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, signuppage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      email,
				FormErrors: ve,
			})
			return
		}

		// A known application failure (e.g. the confirmation mail could not be
		// enqueued): log the internal detail and re-render the form (500) with the
		// user-safe message, so the user can resubmit rather than being sent to a
		// code-entry page for a code that was never delivered.
		//
		// [Ja] 既知のアプリケーション失敗 (例: 確認メールを投入できなかった) のときは、
		// 内部詳細をログに記録し、ユーザー安全なメッセージ付きでフォームを再描画する
		// (500)。届かなかったコードの入力ページに送る代わりに、ユーザーが再申請できる
		// ようにする。
		var ae *model.AppError
		if errors.As(err, &ae) {
			slog.ErrorContext(ctx, ae.LogString())
			// Carry the user-safe message as a form-wide (global) error so the
			// re-rendered form surfaces it through the shared FormErrors alert,
			// the same channel sign-in uses for its credentials error.
			//
			// [Ja] ユーザー安全なメッセージをフォーム全体 (グローバル) のエラーとして
			// 運び、再描画されたフォームが共通の FormErrors アラート経由で表示する。
			// サインインが資格情報エラーに使うのと同じ経路。
			formErrors := model.NewValidationError()
			formErrors.AddGlobal(ae.Error())
			h.renderNew(w, r, http.StatusInternalServerError, signuppage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      email,
				FormErrors: formErrors,
			})
			return
		}

		slog.ErrorContext(ctx, "サインアップ申請に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Carry the new confirmation's id to the code-entry step via a cookie, then
	// send the user there to type the code we just emailed.
	//
	// [Ja] 新しい確認の id を Cookie でコード入力ステップへ運び、たった今メールした
	// コードを入力してもらうためユーザーをそこへ送る。
	h.sessionMgr.SetEmailConfirmationID(w, output.EmailConfirmation.ID)
	http.Redirect(w, r, "/email_confirmation/new", http.StatusSeeOther)
}
