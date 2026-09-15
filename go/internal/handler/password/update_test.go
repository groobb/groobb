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

// patchPasswordはリセットトークンとパスワードフィールドをフォームデータとして運ぶ
// リクエストを組み立て、contextにロケールを設定する。ハンドラーはFormValueで読むため、
// Updateを直接呼ぶときメソッドは無関係 (実サーバーでは _methodオーバーライドがフォームの
// POSTをPATCHにする)。
func patchPassword(token, password, passwordConfirmation string, locale model.Locale) *http.Request {
	form := url.Values{
		"token":                 {token},
		"password":              {password},
		"password_confirmation": {passwordConfirmation},
	}
	req := httptest.NewRequest(http.MethodPatch, "/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// seedResetTokenForUserはパスワードと使えるリセットトークンを持つユーザーを
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

// TestUpdate_Successは、使えるトークンと有効なパスワードがパスワードをリセットし、
// サインインへリダイレクトする (ユーザーは新しいパスワードでサインインする) ことを検証する。
func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newPasswordHandler(t, db)
	rawToken := seedResetTokenForUser(t, db, "oldpassword123")

	rec := httptest.NewRecorder()
	handler.Update(rec, patchPassword(rawToken, "newpassword123", "newpassword123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_in")
	}
}

// TestUpdate_InvalidPasswordRetainsTokenは、フィールドエラー (ここでは確認の不一致)
// が422でフォームを再描画し、トークンをhiddenフィールドに保ち (リンクはまだ有効)、
// レスポンスをno-storeにして、不一致メッセージを表示することを検証する。
func TestUpdate_InvalidPasswordRetainsToken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newPasswordHandler(t, db)
	rawToken := seedResetTokenForUser(t, db, "oldpassword123")

	rec := httptest.NewRecorder()
	handler.Update(rec, patchPassword(rawToken, "newpassword123", "different456", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got := rec.Result().Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "no-store")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "パスワードが一致しません") {
		t.Error("不一致のエラーメッセージが描画されていない")
	}
	// リンクはまだ有効のため、再送信に向けてトークンを保つ。
	if !strings.Contains(body, fmt.Sprintf(`name="token" value="%s"`, rawToken)) {
		t.Error("有効なリンクのトークンが再描画フォームに保たれていない")
	}
}

// TestUpdate_InvalidTokenClearsTokenは、トークンエラー (ここでは未知のトークン) が
// 422・フォーム全体のメッセージで再描画し、失効リンクを再送信できないようhidden
// フィールドからトークンを消去することを検証する。
func TestUpdate_InvalidTokenClearsToken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newPasswordHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Update(rec, patchPassword("no-such-token", "newpassword123", "newpassword123", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="token" value=""`) {
		t.Error("無効なトークンが再描画フォームから消去されていない")
	}
}
