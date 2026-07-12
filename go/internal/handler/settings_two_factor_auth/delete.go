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

// Delete DELETE /settings/two_factor_auth - re-authenticates the request (with the
// current password or a current TOTP code) and, on success, disables 2FA by deleting
// the setting (its secret and recovery codes go with the row), then redirects to the
// settings hub with a completion flash. The route is reached from the disable form
// via the _method=DELETE override. It is registered behind RequireAuth, so the user
// from the context is non-nil. On a validation error (neither credential provided, or
// an incorrect one) the disable form is re-rendered with the message (422). The CSRF
// check is enforced upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Delete DELETE /settings/two_factor_auth - リクエストを再認証し (現在のパスワードか
// 現在の TOTP コード)、成功時に 2FA を無効化して設定を削除し (secret とリカバリーコードは
// 行ごと消える)、完了フラッシュ付きで設定ハブへリダイレクトします。ルートは無効化フォームから
// _method=DELETE のオーバーライドで到達します。RequireAuth の背後に登録されるため、context の
// ユーザーは非 nil です。バリデーションエラー時 (資格情報の未入力、または誤り) は無効化フォームを
// メッセージ付きで再描画します (422)。CSRF 検証は上流の CSRF ミドルウェアが強制するため、
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
			// The submitted password and code are deliberately not echoed back: a
			// password field re-rendered with its value is a credential-leak risk, and
			// a TOTP code is single-use, so the user re-enters whichever one they used.
			//
			// [Ja] 送信されたパスワードとコードは意図的にエコーバックしない。値付きで
			// パスワードフィールドを再描画するのは資格情報の漏えいリスクであり、TOTP コードは
			// 1 回使い切りのため、ユーザーは使った方をもう一度入力する。
			h.renderDisable(w, r, http.StatusUnprocessableEntity, settingstwofactorauthpage.DeletePageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "2 段階認証の無効化に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 2FA is disabled. Existing sessions stay valid (disabling only removes the
	// second factor, it does not sign the user out), so set a completion flash and
	// send the user back to the settings hub, which renders it.
	//
	// [Ja] 2FA が無効になった。既存セッションは有効なまま (無効化は第 2 要素を外すだけで
	// サインアウトはしない) のため、完了フラッシュを設定し、ユーザーをそれを描画する設定ハブへ
	// 戻す。
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_two_factor_auth_disabled"))
	http.Redirect(w, r, templates.SettingsPath().String(), http.StatusSeeOther)
}

// renderDisable renders the 2FA disable confirmation form with the given status and
// data. It is shared by showSetting (200, when 2FA is already enabled) and Delete's
// re-render after a validation error (422). The status is written before rendering,
// so callers pass the final status here rather than setting it separately. The page
// is per-user and behind authentication, so it is marked noindex. It shows no secret
// (only password/code inputs), so unlike the enrollment page it needs no
// Cache-Control: no-store.
//
// [Ja] renderDisable は指定したステータスとデータで 2FA 無効化の確認フォームを描画します。
// showSetting (2FA が既に有効なときの 200) と、Delete のバリデーションエラー後の再描画 (422)
// で共有します。ステータスは描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを
// 渡します。このページはユーザー固有かつ認証の背後にあるため noindex を付けます。secret を
// 表示しない (パスワード / コードの入力欄だけ) ため、登録ページと違い Cache-Control: no-store は
// 不要です。
func (h *Handler) renderDisable(w http.ResponseWriter, r *http.Request, status int, data settingstwofactorauthpage.DeletePageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_two_factor_auth_delete_title")
	meta.Description = i18n.T(ctx, "settings_two_factor_auth_delete_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, settingstwofactorauthpage.Delete(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged, not
		// turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "2 段階認証無効化ページのレンダリングに失敗", "error", err)
	}
}
