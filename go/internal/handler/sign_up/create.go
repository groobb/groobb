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

// Create POST /sign_up - emailを受け付け、メール確認を発行し、コード入力ステップ
// へリダイレクトします。バリデーションエラー時はメッセージ付きでフォームを再描画します
// (422)。CSRF検証は上流のCSRFミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")

	// UseCaseの前にTurnstileトークンを検証する。リクエストレベルのBotゲートとして、
	// 非通過 (未解決 / Botによる偽造ウィジェット) とsiteverifyの失敗のどちらもここで
	// リクエストを止め、UseCaseへは到達させない (確認メールは投入されない)。どちらもwarnで
	// ログする (Groobbはエラートラッカー未連携のためwarnが上限)。ユーザーにはフォーム全体の
	// メッセージとして422で表示する。無効化されたdev / test構成 (シークレットキー空) は検証を
	// 通過するため、そこではこのゲートは透過的に働く。
	if passed, err := h.turnstile.Verify(ctx, r.FormValue("cf-turnstile-response")); err != nil || !passed {
		// 単なる非通過 (passed == false, err == nil) は想定内のBot拒否であり
		// システムエラーではないため、error属性は実際にエラーがあるときだけ付ける。
		// そうしないとログに空のerror=<nil> が残る。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstile検証を通過しなかったためサインアップを受け付けない", attrs...)
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

		// 既知のアプリケーション失敗 (例: 確認メールを投入できなかった) のときは、
		// 内部詳細をログに記録し、ユーザー安全なメッセージ付きでフォームを再描画する
		// (500)。届かなかったコードの入力ページに送る代わりに、ユーザーが再申請できる
		// ようにする。
		var ae *model.AppError
		if errors.As(err, &ae) {
			slog.ErrorContext(ctx, ae.LogString())
			// ユーザー安全なメッセージをフォーム全体 (グローバル) のエラーとして
			// 運び、再描画されたフォームが共通のFormErrorsアラート経由で表示する。
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

	// 新しい確認のidをCookieでコード入力ステップへ運び、たった今メールした
	// コードを入力してもらうためユーザーをそこへ送る。
	h.sessionMgr.SetEmailConfirmationID(w, output.EmailConfirmation.ID)
	http.Redirect(w, r, "/email_confirmation/new", http.StatusSeeOther)
}
