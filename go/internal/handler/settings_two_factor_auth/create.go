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

// Create POST /settings/two_factor_auth - 送信されたTOTPコードをユーザーの登録中の
// 設定に対して検証し、成功時に2FAを有効化して1回使い切りのリカバリーコードを描画します。
// RequireAuthの背後に登録されるため、contextのユーザーは非nilです。バリデーションエラー時
// (コードの未入力・不正・不一致、または登録の不在) は同じsecretからQRを再導出して、
// メッセージ付きで登録フォームを再描画します (422)。CSRF検証は上流のCSRFミドルウェアが
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
			// コードエラー付きで設定ページを再描画する。showSettingは (変わらない)
			// secretを再解決してQRを組み直すため、失敗したコードはエコーバックされず、
			// ユーザーはアプリから新しいコードを読む。2FAが既に有効だった場合 (二重送信が
			// 競合に勝った) は、showSettingが代わりに無効化フォームを表示するため、この
			// フローが行き止まりになることはない。
			h.showSetting(w, r, http.StatusUnprocessableEntity, ve)
			return
		}

		slog.ErrorContext(ctx, "2段階認証の有効化に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 2FAが有効になった。リカバリーコードはここで一度だけ表示され二度と表示されない
	// ため、リダイレクト (安全に運べない) ではなくこのレスポンスに直接描画する。
	h.renderCreated(w, r, out.RecoveryCodes)
}

// renderCreatedは1回使い切りのリカバリーコードを示す「2FAを有効化しました」ページを
// 描画します。このページはユーザー固有かつ認証の背後にあるためnoindexを付けます。ボディが
// 一度だけ表示する平文のリカバリーコードを含むため、Cache-Control: no-storeを付けて、
// どのキャッシュ (ディスク・bfcache) にもコードが残らないようにします。
func (h *Handler) renderCreated(w http.ResponseWriter, r *http.Request, recoveryCodes []string) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_two_factor_auth_create_title")
	meta.Description = i18n.T(ctx, "settings_two_factor_auth_create_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, settingstwofactorauthpage.Create(settingstwofactorauthpage.CreatePageData{
		RecoveryCodes: recoveryCodes,
	})).Render(ctx, w); err != nil {
		// レスポンスボディは部分的に書き込まれている可能性があるため、ここでは
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "リカバリーコードページのレンダリングに失敗", "error", err)
	}
}
