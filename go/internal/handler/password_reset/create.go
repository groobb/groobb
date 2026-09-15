package password_reset

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	passwordresetpage "github.com/groobb/groobb/go/internal/templates/pages/password_reset"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Create POST /password_reset - emailを受け付け、アカウントに属する場合はリセット
// リンクを発行してメールします。成功時は常に同じ "送信しました" の確認を描画するため、
// レスポンスはそのemailが登録済みかどうかを決して明かしません。形式不正のemailは
// メッセージ付きでフォームを再描画します (422)。CSRF検証は上流のCSRFミドルウェアが
// 強制するため、ここでは繰り返しません。本物のシステムエラーはログに記録しつつも送信済み
// ページを表示し、実在アカウントの経路での失敗もアカウントの存在を漏らさないようにします。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")

	// UseCaseの前にTurnstileトークンを検証する。リクエストレベルのBotゲートとして、
	// 非通過 (未解決 / Botによる偽造ウィジェット) とsiteverifyの失敗のどちらもここで
	// リクエストを止め、UseCaseへは到達させない (リセットリンクは発行もメール送信もされない)。
	// どちらもwarnでログする (Groobbはエラートラッカー未連携のためwarnが上限)。ユーザーには
	// フォーム全体のメッセージとして422で表示する。このゲートはアカウント検索の前に走るため、
	// (列挙対策の "送信しました" ページではなく) フォームを再描画しても、そのemailが登録済みか
	// どうかは一切漏れない。列挙の懸念はアカウントを検索した後にのみ生じる。無効化された
	// dev / test構成 (シークレットキー空) は検証を通過するため、そこではこのゲートは透過的に働く。
	if passed, err := h.turnstile.Verify(ctx, r.FormValue("cf-turnstile-response")); err != nil || !passed {
		// 単なる非通過 (passed == false, err == nil) は想定内のBot拒否であり
		// システムエラーではないため、error属性は実際にエラーがあるときだけ付ける。
		// そうしないとログに空のerror=<nil> が残る。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstile検証を通過しなかったためパスワードリセット申請を受け付けない", attrs...)
		formErrors := model.NewValidationError()
		formErrors.AddGlobal(i18n.T(ctx, "validation_turnstile_failed"))
		h.renderNew(w, r, http.StatusUnprocessableEntity, passwordresetpage.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Email:      email,
			FormErrors: formErrors,
		})
		return
	}

	_, err := h.createPasswordResetTokenUC.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  email,
		Locale: i18n.GetLocale(ctx),
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, passwordresetpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      email,
				FormErrors: ve,
			})
			return
		}

		// 本物のシステム障害 (例: データベースに到達できない)。詳細はログに記録するが、
		// 500ではなく同じ送信済みページにフォールスルーする。この経路 (アカウントが見つかった
		// 後にのみ到達する) でだけエラーレスポンスを返すと、そのアドレスがアカウントに属する
		// ことを漏らすため。
		slog.ErrorContext(ctx, "パスワードリセット申請の処理に失敗", "error", err)
	}

	h.renderSent(w, r)
}

// renderSentは送信後の確認ページを描画します。受理されたすべての送信 (実在
// アカウント・未知のemail・内部エラーであっても) で表示するため、レスポンスはその
// アドレスが登録済みかどうかによらず同一になります。
func (h *Handler) renderSent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "password_reset_sent_title")
	meta.Description = i18n.T(ctx, "password_reset_sent_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, passwordresetpage.Sent()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "パスワードリセット送信完了ページのレンダリングに失敗", "error", err)
	}
}
