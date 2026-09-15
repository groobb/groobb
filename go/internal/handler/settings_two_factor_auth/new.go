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

// New GET /settings/two_factor_auth/new - 2FA設定ページを表示します。2FAが無効なら
// 登録フォーム (ユーザーのsecretのQRコードと手動入力キー、そして認証アプリのコードを
// 確認するフィールド) を、既に有効なら無効化の確認フォームを表示します。RequireAuthの
// 背後に登録されるため、contextのユーザーは非nilです。登録 / 無効化の選択はshowSettingで
// 行います。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	h.showSetting(w, r, http.StatusOK, nil)
}

// showSettingはユーザーの2FAの状態を解決し、指定したステータスで対応するページを
// 描画します。2FAが既に有効なら無効化の確認フォームを、そうでなければ登録用secretを解決し
// (登録中のものを再利用するか新規発行する)、otpauth URIとそのQRコードを組み立てて設定
// フォームを表示します。New (200) と、Createのバリデーションエラー後の再描画 (422) で共有し、
// 同じ冪等なステップで状態を解決することで分岐を1箇所に保ちます。渡されるフォームエラーは
// 登録フォーム用 (有効化のエラー) であり、2FAが有効だった場合はそれを伴わずに無効化フォームを
// 表示します。レスポンス書き込み前の失敗は500になります。
func (h *Handler) showSetting(w http.ResponseWriter, r *http.Request, status int, formErrors *model.ValidationError) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	out, err := h.prepareUC.Execute(ctx, usecase.PrepareTwoFactorAuthInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "2段階認証の設定準備に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if out.AlreadyEnabled {
		// 既に有効: 再登録しない (アクティブなsecretとリカバリーコードを上書きして
		// しまうため)。代わりに無効化の確認フォームを表示し、この設定ページを2FAを無効化
		// する唯一の場所にする。
		h.renderDisable(w, r, status, settingstwofactorauthpage.DeletePageData{
			CSRFToken: middleware.CSRFTokenFromContext(ctx),
		})
		return
	}

	otpauthURL, err := auth.BuildOTPAuthURL(out.Secret, user.Email)
	if err != nil {
		slog.ErrorContext(ctx, "otpauth URIの生成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	qrDataURI, err := qrcode.PNGDataURI(otpauthURL)
	if err != nil {
		slog.ErrorContext(ctx, "QRコードの生成に失敗", "error", err)
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

// renderNewは指定したステータスとデータで2FA登録フォームを描画します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータス (Newからは200、
// Createの再描画からは422) を渡します。このページはユーザー固有かつ認証の背後にあるため
// noindexを付けます。ボディが手動入力用に平文のTOTP secretを表示するため、
// Cache-Control: no-storeを付けて、どのキャッシュ (ディスク・bfcache) にもsecretが
// 残らないようにします (リカバリーコードのレスポンスと揃えます)。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data settingstwofactorauthpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_two_factor_auth_new_title")
	meta.Description = i18n.T(ctx, "settings_two_factor_auth_new_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, settingstwofactorauthpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "2段階認証設定ページのレンダリングに失敗", "error", err)
	}
}
