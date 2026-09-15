package settings_email

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	settingsemailpage "github.com/groobb/groobb/go/internal/templates/pages/settings_email"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Update PATCH /settings/email - 新しいemailと現在のパスワードを受け付け、新しい
// アドレス宛にメール確認を発行し、コード入力ステップへリダイレクトします。ルートはHTML
// フォームから _method=PATCHのオーバーライドで到達します。RequireAuthの背後に登録される
// ため、contextのユーザーは非nilです。バリデーションエラー時はメッセージ付きでフォームを
// 再描画します (422)。確認メールを投入できない (AppError) ときはフォーム全体のメッセージ付きで
// 再描画し (500)、届かなかったコードの入力ページに送る代わりにユーザーが再申請できる
// ようにします。CSRF検証は上流のCSRFミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	newEmail := r.FormValue("email")
	currentPassword := r.FormValue("current_password")

	_, err := h.createEmailChangeUC.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID:          user.ID,
		NewEmail:        newEmail,
		CurrentPassword: currentPassword,
		Locale:          i18n.GetLocale(ctx),
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			// 試した新しいemailはエコーバックしてユーザーが打ち直さずに済むように
			// する。現在のパスワードは資格情報の漏えいリスクのため意図的にエコーせず、
			// ユーザーに再入力させる。
			h.renderEdit(w, r, http.StatusUnprocessableEntity, settingsemailpage.EditPageData{
				CSRFToken:    middleware.CSRFTokenFromContext(ctx),
				CurrentEmail: user.Email,
				NewEmail:     newEmail,
				FormErrors:   ve,
			})
			return
		}

		// 既知のアプリケーション失敗 (確認メールを投入できなかった) のときは、内部
		// 詳細をログに記録し、ユーザー安全なメッセージをフォーム全体のエラーとして付けて
		// フォームを再描画する (500)。ユーザーが再申請できるようにする。
		var ae *model.AppError
		if errors.As(err, &ae) {
			slog.ErrorContext(ctx, ae.LogString())
			formErrors := model.NewValidationError()
			formErrors.AddGlobal(ae.Error())
			h.renderEdit(w, r, http.StatusInternalServerError, settingsemailpage.EditPageData{
				CSRFToken:    middleware.CSRFTokenFromContext(ctx),
				CurrentEmail: user.Email,
				NewEmail:     newEmail,
				FormErrors:   formErrors,
			})
			return
		}

		slog.ErrorContext(ctx, "メールアドレス変更申請に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// たった今新しいアドレスにメールしたコードを入力してもらうため、ユーザーを
	// コード入力ステップへ送る。確認ステップ (後続フェーズで作成) は保留中の確認を
	// セッションのユーザーから解決するため、ここでは受け渡しCookieを設定しない。
	http.Redirect(w, r, "/settings/email/confirmation/new", http.StatusSeeOther)
}
