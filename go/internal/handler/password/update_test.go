package password_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// patchPassword builds a request carrying the reset token and password fields as
// form data, with the locale set in its context. The handler reads them with
// FormValue, so the method is irrelevant when Update is called directly (in the
// running server the _method override turns the form POST into a PATCH).
//
// [Ja] patchPassword はリセットトークンとパスワードフィールドをフォームデータとして運ぶ
// リクエストを組み立て、context にロケールを設定する。ハンドラーは FormValue で読むため、
// Update を直接呼ぶときメソッドは無関係 (実サーバーでは _method オーバーライドがフォームの
// POST を PATCH にする)。
func patchPassword(token, password, passwordConfirmation, locale string) *http.Request {
	form := url.Values{
		"token":                 {token},
		"password":              {password},
		"password_confirmation": {passwordConfirmation},
	}
	req := httptest.NewRequest(http.MethodPatch, "/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// seedResetTokenForUser creates a user with a password and a usable reset token,
// returning the plaintext token to submit.
//
// [Ja] seedResetTokenForUser はパスワードと使えるリセットトークンを持つユーザーを
// 作成し、送信する平文トークンを返す。
func seedResetTokenForUser(t *testing.T, db *database.DB, password string) string {
	t.Helper()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	tokenRepo := repository.NewPasswordResetTokenRepository(db)

	email := "pw-h@example.com"
	user, err := userRepo.Create(ctx, repository.CreateUserInput{Email: email, Atname: testutil.UniqueAtname(db), Locale: "ja", TimeZone: "Asia/Tokyo"})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	digest, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{UserID: user.ID, PasswordDigest: digest}); err != nil {
		t.Fatalf("パスワード資格情報の作成に失敗: %v", err)
	}

	rawToken := "h-token"
	if _, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      user.ID,
		TokenDigest: auth.HashToken(rawToken),
		ExpiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	}); err != nil {
		t.Fatalf("トークンの作成に失敗: %v", err)
	}
	return rawToken
}

// TestUpdate_Success verifies that a usable token and a valid password reset the
// password and redirect to sign-in (the user signs in with the new password).
//
// [Ja] TestUpdate_Success は、使えるトークンと有効なパスワードがパスワードをリセットし、
// サインインへリダイレクトする (ユーザーは新しいパスワードでサインインする) ことを検証する。
func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newPasswordHandler(t, db)
	rawToken := seedResetTokenForUser(t, db, "oldpassword123")

	rec := httptest.NewRecorder()
	handler.Update(rec, patchPassword(rawToken, "newpassword123", "newpassword123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in" {
		t.Errorf("Location = %q, want %q", loc, "/sign_in")
	}
}

// TestUpdate_InvalidPasswordRetainsToken verifies that a field error (here, a
// mismatched confirmation) re-renders the form with 422, keeps the token in the
// hidden field (the link is still valid), marks the response as no-store, and shows
// the mismatch message.
//
// [Ja] TestUpdate_InvalidPasswordRetainsToken は、フィールドエラー (ここでは確認の不一致)
// が 422 でフォームを再描画し、トークンを hidden フィールドに保ち (リンクはまだ有効)、
// レスポンスを no-store にして、不一致メッセージを表示することを検証する。
func TestUpdate_InvalidPasswordRetainsToken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newPasswordHandler(t, db)
	rawToken := seedResetTokenForUser(t, db, "oldpassword123")

	rec := httptest.NewRecorder()
	handler.Update(rec, patchPassword(rawToken, "newpassword123", "different456", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got := rec.Result().Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "パスワードが一致しません") {
		t.Error("不一致のエラーメッセージが描画されていない")
	}
	// The link is still valid, so the token is kept for re-submission.
	//
	// [Ja] リンクはまだ有効のため、再送信に向けてトークンを保つ。
	if !strings.Contains(body, fmt.Sprintf(`name="token" value="%s"`, rawToken)) {
		t.Error("有効なリンクのトークンが再描画フォームに保たれていない")
	}
}

// TestUpdate_InvalidTokenClearsToken verifies that a token error (here, an
// unknown token) re-renders the form with 422, a form-wide message, and the token
// cleared from the hidden field so the dead link cannot be re-submitted.
//
// [Ja] TestUpdate_InvalidTokenClearsToken は、トークンエラー (ここでは未知のトークン) が
// 422・フォーム全体のメッセージで再描画し、失効リンクを再送信できないよう hidden
// フィールドからトークンを消去することを検証する。
func TestUpdate_InvalidTokenClearsToken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newPasswordHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Update(rec, patchPassword("no-such-token", "newpassword123", "newpassword123", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="token" value=""`) {
		t.Error("無効なトークンが再描画フォームから消去されていない")
	}
}
