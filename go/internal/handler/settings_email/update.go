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

// Update PATCH /settings/email - accepts a new email and the current password,
// issues an email confirmation for the new address, and redirects to the
// code-entry step. The route is reached from the HTML form via the _method=PATCH
// override. It is registered behind RequireAuth, so the user from the context is
// non-nil. On a validation error the form is re-rendered with the messages (422);
// when the confirmation mail cannot be enqueued (an AppError) the form is
// re-rendered with a form-wide message (500) so the user can retry rather than
// being sent to a code-entry page for a code that was never delivered. The CSRF
// check is enforced upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Update PATCH /settings/email - 新しい email と現在のパスワードを受け付け、新しい
// アドレス宛にメール確認を発行し、コード入力ステップへリダイレクトします。ルートは HTML
// フォームから _method=PATCH のオーバーライドで到達します。RequireAuth の背後に登録される
// ため、context のユーザーは非 nil です。バリデーションエラー時はメッセージ付きでフォームを
// 再描画します (422)。確認メールを投入できない (AppError) ときはフォーム全体のメッセージ付きで
// 再描画し (500)、届かなかったコードの入力ページに送る代わりにユーザーが再申請できる
// ようにします。CSRF 検証は上流の CSRF ミドルウェアが強制するため、ここでは繰り返しません。
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
			// Echo the attempted new email back so the user does not retype it; the
			// current password is deliberately not echoed (a credential-leak risk),
			// so the user re-enters it.
			//
			// [Ja] 試した新しい email はエコーバックしてユーザーが打ち直さずに済むように
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

		// A known application failure (the confirmation mail could not be enqueued):
		// log the internal detail and re-render the form (500) with the user-safe
		// message as a form-wide error, so the user can resubmit.
		//
		// [Ja] 既知のアプリケーション失敗 (確認メールを投入できなかった) のときは、内部
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

	// Send the user to the code-entry step to type the code just emailed to the
	// new address. The confirm step (built in a later phase) resolves the pending
	// confirmation from the session user, so no handoff cookie is set here.
	//
	// [Ja] たった今新しいアドレスにメールしたコードを入力してもらうため、ユーザーを
	// コード入力ステップへ送る。確認ステップ (後続フェーズで作成) は保留中の確認を
	// セッションのユーザーから解決するため、ここでは受け渡し Cookie を設定しない。
	http.Redirect(w, r, "/settings/email/confirmation/new", http.StatusSeeOther)
}
