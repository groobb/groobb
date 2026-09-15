package settings_two_factor_auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	settingstwofactorauthpage "github.com/groobb/groobb/go/internal/templates/pages/settings_two_factor_auth"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Delete DELETE /settings/two_factor_auth - リクエストを再認証し (現在のパスワードか
// 現在のTOTPコード)、成功時に2FAを無効化して設定を削除し (secretとリカバリーコードは
// 行ごと消える)、完了フラッシュ付きで設定ハブへリダイレクトします。ルートは無効化フォームから
// _method=DELETEのオーバーライドで到達します。RequireAuthの背後に登録されるため、contextの
// ユーザーは非nilです。バリデーションエラー時 (資格情報の未入力、または誤り) は無効化フォームを
// メッセージ付きで再描画します (422)。CSRF検証は上流のCSRFミドルウェアが強制するため、
// ここでは繰り返しません。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	if err := h.disableUC.Execute(ctx, usecase.DisableTwoFactorAuthInput{
		UserID:          user.ID,
		CurrentPassword: r.FormValue("current_password"),
		Code:            r.FormValue("code"),
	}); err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			// 送信されたパスワードとコードは意図的にエコーバックしない。値付きで
			// パスワードフィールドを再描画するのは資格情報の漏えいリスクであり、TOTPコードは
			// 1回使い切りのため、ユーザーは使った方をもう一度入力する。
			h.renderDisable(w, r, http.StatusUnprocessableEntity, settingstwofactorauthpage.DeletePageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "2段階認証の無効化に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 2FAが無効になった。既存セッションは有効なまま (無効化は第2要素を外すだけで
	// サインアウトはしない) のため、完了フラッシュを設定し、ユーザーをそれを描画する設定ハブへ
	// 戻す。
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_two_factor_auth_disabled"))
	http.Redirect(w, r, templates.SettingsPath().String(), http.StatusSeeOther)
}

// renderDisableは指定したステータスとデータで2FA無効化の確認フォームを描画します。
// showSetting (2FAが既に有効なときの200) と、Deleteのバリデーションエラー後の再描画 (422)
// で共有します。ステータスは描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを
// 渡します。このページはユーザー固有かつ認証の背後にあるためnoindexを付けます。secretを
// 表示しない (パスワード / コードの入力欄だけ) ため、登録ページと違いCache-Control: no-storeは
// 不要です。
func (h *Handler) renderDisable(w http.ResponseWriter, r *http.Request, status int, data settingstwofactorauthpage.DeletePageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_two_factor_auth_delete_title")
	meta.Description = i18n.T(ctx, "settings_two_factor_auth_delete_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, settingstwofactorauthpage.Delete(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "2段階認証無効化ページのレンダリングに失敗", "error", err)
	}
}
