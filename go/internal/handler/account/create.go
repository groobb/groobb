package account

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/clientip"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	accountpage "github.com/groobb/groobb/go/internal/templates/pages/account"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /account - creates the account from the verified confirmation and
// the submitted password, then signs the user in by issuing a session. A request
// without the handoff cookie has no confirmation to build from, so it is sent
// back to sign-up. On a validation error the form is re-rendered with the
// messages (422). When the verified confirmation is gone (an AppError), the
// stale cookie is cleared and the user is returned to sign-up to start over. The
// CSRF check is enforced upstream by the CSRF middleware, so it is not repeated
// here.
//
// [Ja] Create POST /account - 検証済みの確認と送信されたパスワードからアカウントを作成し、
// セッションを発行してユーザーをサインインさせます。受け渡し Cookie の無いリクエストは
// 作成の元となる確認が無いため、サインアップへ戻します。バリデーションエラー時はメッセージ
// 付きでフォームを再描画します (422)。検証済みの確認が失われている (AppError) ときは、
// 失効した Cookie を消去しユーザーをサインアップのやり直しへ戻します。CSRF 検証は上流の
// CSRF ミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := h.sessionMgr.GetEmailConfirmationID(r)
	if !ok {
		http.Redirect(w, r, "/sign_up", http.StatusSeeOther)
		return
	}

	atname := r.FormValue("atname")
	password := r.FormValue("password")
	passwordConfirmation := r.FormValue("password_confirmation")

	accountOutput, err := h.createAccountUC.Execute(ctx, usecase.CreateAccountInput{
		EmailConfirmationID:  id,
		Atname:               atname,
		Password:             password,
		PasswordConfirmation: passwordConfirmation,
		Locale:               i18n.GetLocale(ctx),
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			// Echo the submitted atname back so the user does not retype it; the
			// password fields are deliberately not echoed (a credential-leak risk),
			// so the user re-enters those.
			//
			// [Ja] 送信された atname はエコーバックしてユーザーが打ち直さずに済むようにする。
			// パスワードフィールドは資格情報の漏えいリスクのため意図的にエコーせず、そちらは
			// ユーザーに再入力させる。
			h.renderNew(w, r, http.StatusUnprocessableEntity, accountpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Atname:     atname,
				FormErrors: ve,
			})
			return
		}

		// The verified confirmation is gone (stale handoff, already used, or never
		// verified). Clear the stale cookie and send the user back to start
		// sign-up over, rather than rendering an error page for a flow they can
		// simply restart.
		//
		// [Ja] 検証済みの確認が失われている (受け渡しが失効・使用済み・未検証)。失効した
		// Cookie を消去し、単に再開できるフローにエラーページを出す代わりに、ユーザーを
		// サインアップのやり直しへ戻す。
		var ae *model.AppError
		if errors.As(err, &ae) {
			slog.WarnContext(ctx, ae.LogString())
			h.sessionMgr.DeleteEmailConfirmationID(w)
			http.Redirect(w, r, "/sign_up", http.StatusSeeOther)
			return
		}

		slog.ErrorContext(ctx, "アカウント作成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Sign the new user in: create a session and store its token in the session
	// cookie. A failure here means the account exists but sign-in did not
	// complete; surface it as a 500 so the user is not silently left signed out
	// with no feedback.
	//
	// [Ja] 新規ユーザーをサインインさせる: セッションを作成しそのトークンをセッション
	// Cookie に格納する。ここでの失敗はアカウントは存在するがサインインが完了しなかった
	// ことを意味するため、500 として表面化し、ユーザーがフィードバック無しに黙って
	// 未サインインのまま放置されないようにする。
	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    accountOutput.User.ID,
		IPAddress: clientip.GetClientIP(r, h.cfg.TrustedProxies),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// The account is committed but sign-in failed. Clear the handoff cookie so a
		// retry does not replay the now-spent confirmation: re-running account
		// creation would re-create the same user, hit the users.email UNIQUE
		// constraint, and loop on 500. Dropping the cookie makes a retry fall back to
		// /sign_up instead, and the already-created account can be reached via
		// sign-in once that flow exists (phase 4-3).
		//
		// [Ja] アカウントはコミット済みだがサインインに失敗した。受け渡し Cookie を消去し、
		// 再送信が消費済みの確認を再生しないようにする。アカウント作成をやり直すと同じ
		// ユーザーを再作成して users.email の UNIQUE 制約に当たり、500 をループするため。
		// Cookie を落とすことで再送信は /sign_up へフォールバックし、作成済みアカウントは
		// サインインフロー (フェーズ 4-3) の整備後にそこからたどり着ける。
		h.sessionMgr.DeleteEmailConfirmationID(w)
		slog.ErrorContext(ctx, "セッションの作成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)

	// Clear the handoff cookie now that the confirmation has served its purpose,
	// so a later request cannot replay it.
	//
	// [Ja] 確認が役目を終えたので受け渡し Cookie を消去し、後続のリクエストがそれを
	// 再生できないようにする。
	h.sessionMgr.DeleteEmailConfirmationID(w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
