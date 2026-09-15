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

// Create POST /account - 検証済みの確認と送信されたパスワードからアカウントを作成し、
// セッションを発行してユーザーをサインインさせます。受け渡しCookieの無いリクエストは
// 作成の元となる確認が無いため、サインアップへ戻します。バリデーションエラー時はメッセージ
// 付きでフォームを再描画します (422)。検証済みの確認が失われている (AppError) ときは、
// 失効したCookieを消去しユーザーをサインアップのやり直しへ戻します。CSRF検証は上流の
// CSRFミドルウェアが強制するため、ここでは繰り返しません。
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
			// 送信されたatnameはエコーバックしてユーザーが打ち直さずに済むようにする。
			// パスワードフィールドは資格情報の漏えいリスクのため意図的にエコーせず、そちらは
			// ユーザーに再入力させる。
			h.renderNew(w, r, http.StatusUnprocessableEntity, accountpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Atname:     atname,
				FormErrors: ve,
			})
			return
		}

		// 検証済みの確認が失われている (受け渡しが失効・使用済み・未検証)。失効した
		// Cookieを消去し、単に再開できるフローにエラーページを出す代わりに、ユーザーを
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

	// 新規ユーザーをサインインさせる: セッションを作成しそのトークンをセッション
	// Cookieに格納する。ここでの失敗はアカウントは存在するがサインインが完了しなかった
	// ことを意味するため、500として表面化し、ユーザーがフィードバック無しに黙って
	// 未サインインのまま放置されないようにする。
	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    accountOutput.User.ID,
		IPAddress: clientip.GetClientIP(r, h.cfg.TrustedProxies),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// アカウントはコミット済みだがサインインに失敗した。受け渡しCookieを消去し、
		// 再送信が消費済みの確認を再生しないようにする。アカウント作成をやり直すと同じ
		// ユーザーを再作成してusers.emailのUNIQUE制約に当たり、500をループするため。
		// Cookieを落とすことで再送信は /sign_upへフォールバックし、作成済みアカウントは
		// サインインフロー (フェーズ4-3) の整備後にそこからたどり着ける。
		h.sessionMgr.DeleteEmailConfirmationID(w)
		slog.ErrorContext(ctx, "セッションの作成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)

	// 確認が役目を終えたので受け渡しCookieを消去し、後続のリクエストがそれを
	// 再生できないようにする。
	h.sessionMgr.DeleteEmailConfirmationID(w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
