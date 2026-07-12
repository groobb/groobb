package settings_two_factor_auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	settingstwofactorauthpage "github.com/groobb/groobb/go/internal/templates/pages/settings_two_factor_auth"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Create POST /settings/two_factor_auth - verifies the submitted TOTP code against
// the user's in-progress enrollment and, on success, enables 2FA and renders the
// one-time recovery codes. It is registered behind RequireAuth, so the user from the
// context is non-nil. On a validation error (a missing, malformed, or incorrect
// code, or a missing enrollment) the enrollment form is re-rendered with the message
// (422), re-deriving the QR from the same secret. The CSRF check is enforced
// upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Create POST /settings/two_factor_auth - 送信された TOTP コードをユーザーの登録中の
// 設定に対して検証し、成功時に 2FA を有効化して 1 回使い切りのリカバリーコードを描画します。
// RequireAuth の背後に登録されるため、context のユーザーは非 nil です。バリデーションエラー時
// (コードの未入力・不正・不一致、または登録の不在) は同じ secret から QR を再導出して、
// メッセージ付きで登録フォームを再描画します (422)。CSRF 検証は上流の CSRF ミドルウェアが
// 強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	code := r.FormValue("code")

	out, err := h.enableUC.Execute(ctx, usecase.EnableTwoFactorAuthInput{
		UserID: user.ID,
		Code:   code,
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			// Re-render the settings page with the code error. showSetting re-resolves
			// the (unchanged) secret and rebuilds the QR, so the failed code is not
			// echoed back and the user reads a fresh one from their app. If 2FA turns
			// out to be already enabled (a double submit won the race), showSetting
			// shows the disable form instead, so the flow is never a dead end.
			//
			// [Ja] コードエラー付きで設定ページを再描画する。showSetting は (変わらない)
			// secret を再解決して QR を組み直すため、失敗したコードはエコーバックされず、
			// ユーザーはアプリから新しいコードを読む。2FA が既に有効だった場合 (二重送信が
			// 競合に勝った) は、showSetting が代わりに無効化フォームを表示するため、この
			// フローが行き止まりになることはない。
			h.showSetting(w, r, http.StatusUnprocessableEntity, ve)
			return
		}

		slog.ErrorContext(ctx, "2 段階認証の有効化に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 2FA is enabled. The recovery codes are shown here once and never again, so
	// they are rendered directly on this response rather than via a redirect (a
	// redirect could not carry them safely).
	//
	// [Ja] 2FA が有効になった。リカバリーコードはここで一度だけ表示され二度と表示されない
	// ため、リダイレクト (安全に運べない) ではなくこのレスポンスに直接描画する。
	h.renderCreated(w, r, out.RecoveryCodes)
}

// renderCreated renders the "2FA enabled" page showing the one-time recovery codes.
// The page is per-user and behind authentication, so it is marked noindex. Because
// the body carries plaintext recovery codes shown only once, it is sent with
// Cache-Control: no-store so no cache (disk or bfcache) retains the codes.
//
// [Ja] renderCreated は 1 回使い切りのリカバリーコードを示す「2FA を有効化しました」ページを
// 描画します。このページはユーザー固有かつ認証の背後にあるため noindex を付けます。ボディが
// 一度だけ表示する平文のリカバリーコードを含むため、Cache-Control: no-store を付けて、
// どのキャッシュ (ディスク・bfcache) にもコードが残らないようにします。
func (h *Handler) renderCreated(w http.ResponseWriter, r *http.Request, recoveryCodes []string) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_two_factor_auth_create_title")
	meta.Description = i18n.T(ctx, "settings_two_factor_auth_create_description")
	meta.NoIndex = true

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, settingstwofactorauthpage.Create(settingstwofactorauthpage.CreatePageData{
		RecoveryCodes: recoveryCodes,
	})).Render(ctx, w); err != nil {
		// The response body may be partially written, so this can only be logged.
		//
		// [Ja] レスポンスボディは部分的に書き込まれている可能性があるため、ここでは
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "リカバリーコードページのレンダリングに失敗", "error", err)
	}
}
