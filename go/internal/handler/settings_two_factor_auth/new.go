package settings_two_factor_auth

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/qrcode"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	settingstwofactorauthpage "github.com/groobb/groobb/go/internal/templates/pages/settings_two_factor_auth"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /settings/two_factor_auth/new - shows the 2FA settings page: the
// enrollment form (the QR code and manual-entry key for the user's secret, and a
// field to confirm a code from their authenticator app) when 2FA is off, or the
// disable confirmation form when it is already on. It is registered behind
// RequireAuth, so the user from the context is non-nil. The enroll/disable choice is
// made in showSetting.
//
// [Ja] New GET /settings/two_factor_auth/new - 2FA 設定ページを表示します。2FA が無効なら
// 登録フォーム (ユーザーの secret の QR コードと手動入力キー、そして認証アプリのコードを
// 確認するフィールド) を、既に有効なら無効化の確認フォームを表示します。RequireAuth の
// 背後に登録されるため、context のユーザーは非 nil です。登録 / 無効化の選択は showSetting で
// 行います。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	h.showSetting(w, r, http.StatusOK, nil)
}

// showSetting resolves the user's 2FA state and renders the matching page at the
// given status: when 2FA is already enabled it shows the disable confirmation form,
// otherwise it resolves the enrollment secret (reusing an in-progress one or issuing
// a fresh one), builds the otpauth URI and its QR code, and shows the setup form. It
// is shared by New (200) and Create's re-render after a validation error (422);
// resolving state through the same idempotent step keeps the branching in one place.
// The passed form errors apply to the enrollment form only (they are the enable
// error); when 2FA turns out to be enabled the disable form is shown without them.
// Any failure before the response is written becomes a 500.
//
// [Ja] showSetting はユーザーの 2FA の状態を解決し、指定したステータスで対応するページを
// 描画します。2FA が既に有効なら無効化の確認フォームを、そうでなければ登録用 secret を解決し
// (登録中のものを再利用するか新規発行する)、otpauth URI とその QR コードを組み立てて設定
// フォームを表示します。New (200) と、Create のバリデーションエラー後の再描画 (422) で共有し、
// 同じ冪等なステップで状態を解決することで分岐を 1 箇所に保ちます。渡されるフォームエラーは
// 登録フォーム用 (有効化のエラー) であり、2FA が有効だった場合はそれを伴わずに無効化フォームを
// 表示します。レスポンス書き込み前の失敗は 500 になります。
func (h *Handler) showSetting(w http.ResponseWriter, r *http.Request, status int, formErrors *model.ValidationError) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	out, err := h.prepareUC.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "2 段階認証の設定準備に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if out.AlreadyEnabled {
		// Already enabled: do not re-enroll (which would overwrite the active secret
		// and recovery codes). Show the disable confirmation form instead, so the
		// settings page is the one place to turn 2FA off.
		//
		// [Ja] 既に有効: 再登録しない (アクティブな secret とリカバリーコードを上書きして
		// しまうため)。代わりに無効化の確認フォームを表示し、この設定ページを 2FA を無効化
		// する唯一の場所にする。
		h.renderDisable(w, r, status, settingstwofactorauthpage.DeletePageData{
			CSRFToken: middleware.CSRFTokenFromContext(ctx),
		})
		return
	}

	otpauthURL, err := auth.BuildOTPAuthURL(out.Secret, user.Email)
	if err != nil {
		slog.ErrorContext(ctx, "otpauth URI の生成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	qrDataURI, err := qrcode.PNGDataURI(otpauthURL)
	if err != nil {
		slog.ErrorContext(ctx, "QR コードの生成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderNew(w, r, status, settingstwofactorauthpage.NewPageData{
		CSRFToken:     middleware.CSRFTokenFromContext(ctx),
		QRCodeDataURI: qrDataURI,
		Secret:        out.Secret,
		FormErrors:    formErrors,
	})
}

// renderNew renders the 2FA enrollment form with the given status and data. The
// status is written before rendering, so callers pass the final status (200 from
// New, 422 from Create's re-render) here rather than setting it separately. The page
// is per-user and behind authentication, so it is marked noindex. Because the body
// shows the plaintext TOTP secret for manual entry, it is sent with Cache-Control:
// no-store so no cache (disk or bfcache) retains the secret, matching the
// recovery-codes response.
//
// [Ja] renderNew は指定したステータスとデータで 2FA 登録フォームを描画します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータス (New からは 200、
// Create の再描画からは 422) を渡します。このページはユーザー固有かつ認証の背後にあるため
// noindex を付けます。ボディが手動入力用に平文の TOTP secret を表示するため、
// Cache-Control: no-store を付けて、どのキャッシュ (ディスク・bfcache) にも secret が
// 残らないようにします (リカバリーコードのレスポンスと揃えます)。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data settingstwofactorauthpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_two_factor_auth_new_title")
	meta.Description = i18n.T(ctx, "settings_two_factor_auth_new_description")
	meta.NoIndex = true

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, settingstwofactorauthpage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged, not
		// turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "2 段階認証設定ページのレンダリングに失敗", "error", err)
	}
}
