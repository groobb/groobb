package settings_two_factor_auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// deleteTwoFactorAuth builds a DELETE /settings/two_factor_auth request carrying the
// current password and TOTP code as form data (each omitted when empty), the user in
// the context (as RequireAuth would place it), and the locale set. The form is parsed
// while the request is still a POST and only then is the method switched to DELETE,
// mirroring the method-override middleware: Go's ParseForm reads the body only for
// POST/PUT/PATCH, so a request constructed directly as DELETE would leave FormValue
// empty.
//
// [Ja] deleteTwoFactorAuth は現在のパスワードと TOTP コードをフォームデータ (空のものは
// 省く) として運ぶ DELETE /settings/two_factor_auth リクエストを組み立て、(RequireAuth が
// 置くように) context のユーザーとロケールを載せる。フォームはリクエストがまだ POST のうちに
// 解析し、その後でメソッドを DELETE に切り替える。これはメソッドオーバーライドミドルウェアの
// 再現で、Go の ParseForm は POST/PUT/PATCH のときだけボディを読むため、最初から DELETE として
// 組み立てると FormValue が空になる。
func deleteTwoFactorAuth(user *model.User, currentPassword, code, locale string) *http.Request {
	form := url.Values{}
	if currentPassword != "" {
		form.Set("current_password", currentPassword)
	}
	if code != "" {
		form.Set("code", code)
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		panic(err)
	}
	req.Method = http.MethodDelete
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// TestDelete_SuccessWithPassword verifies that DELETE /settings/two_factor_auth with
// the correct current password disables 2FA (deletes the setting), sets the
// completion flash, and redirects to the settings hub.
//
// [Ja] TestDelete_SuccessWithPassword は、正しい現在のパスワード付きの
// DELETE /settings/two_factor_auth が 2FA を無効化し (設定を削除する)、完了フラッシュを設定し、
// 設定ハブへリダイレクトすることを検証する。
func TestDelete_SuccessWithPassword(t *testing.T) {
	t.Parallel()

	h, repo, user, tx := setupTwoFactorAuthHandler(t)
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(user.ID).WithEnabled(true).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(user.ID).Build()

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteTwoFactorAuth(user, testutil.DefaultBuilderPassword, "", i18n.LangJa))

	assertDisabled(t, rec, repo, user.ID)
}

// TestDelete_SuccessWithCode verifies that DELETE /settings/two_factor_auth with a
// correct current TOTP code (and no password) disables 2FA, so a user who cannot
// recall their password can still turn it off with their authenticator.
//
// [Ja] TestDelete_SuccessWithCode は、正しい現在の TOTP コード (パスワードなし) 付きの
// DELETE /settings/two_factor_auth が 2FA を無効化し、パスワードを思い出せないユーザーも
// 認証アプリで無効化できることを検証する。
func TestDelete_SuccessWithCode(t *testing.T) {
	t.Parallel()

	h, repo, user, tx := setupTwoFactorAuthHandler(t)
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(user.ID).WithEnabled(true).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteTwoFactorAuth(user, "", code, i18n.LangJa))

	assertDisabled(t, rec, repo, user.ID)
}

// TestDelete_ValidationError verifies that a wrong current password (and no code)
// re-renders the disable form with 422 and the incorrect-credentials message, and
// leaves 2FA enabled.
//
// [Ja] TestDelete_ValidationError は、誤った現在のパスワード (コードなし) が無効化フォームを
// 422 と資格情報誤りのメッセージで再描画し、2FA を有効なまま残すことを検証する。
func TestDelete_ValidationError(t *testing.T) {
	t.Parallel()

	h, repo, user, tx := setupTwoFactorAuthHandler(t)
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(user.ID).WithEnabled(true).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(user.ID).Build()

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteTwoFactorAuth(user, "wrongpassword", "", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "現在のパスワードか認証コードが正しくありません") {
		t.Error("資格情報誤りのエラーメッセージが描画されていない")
	}
	// The disable form is re-rendered (still driving DELETE /settings/two_factor_auth).
	//
	// [Ja] 無効化フォームが再描画される (引き続き DELETE /settings/two_factor_auth を動かす)。
	if !strings.Contains(body, `action="/settings/two_factor_auth"`) {
		t.Error("無効化フォームが再描画されていない")
	}

	// 2FA is left enabled: the setting still exists and is enabled.
	//
	// [Ja] 2FA は有効なまま残る: 設定はまだ存在し有効である。
	stored, err := repo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("バリデーション失敗時に 2FA 設定が削除された")
	}
	if !stored.Enabled {
		t.Error("バリデーション失敗時に 2FA が無効化された")
	}
}

// assertDisabled checks the shared success expectations for a disable request: a
// 303 redirect to the settings hub, a success flash carrying the disabled message,
// and the 2FA setting deleted.
//
// [Ja] assertDisabled は無効化リクエストの成功時の共通期待を検証する。設定ハブへの 303
// リダイレクト、無効化メッセージを運ぶ成功フラッシュ、そして 2FA 設定が削除されていること。
func assertDisabled(t *testing.T, rec *httptest.ResponseRecorder, repo *repository.UserTwoFactorAuthRepository, userID model.UserID) {
	t.Helper()

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/settings" {
		t.Errorf("Location = %q, want %q", loc, "/settings")
	}

	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("flash type = %q, want %q", flash.Type, session.FlashSuccess)
	}
	want := i18n.T(i18n.SetLocale(context.Background(), i18n.LangJa), "flash_two_factor_auth_disabled")
	if flash.Message != want {
		t.Errorf("flash message = %q, want %q", flash.Message, want)
	}

	// The setting is deleted, so 2FA is off.
	//
	// [Ja] 設定が削除され、2FA は無効になる。
	stored, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored != nil {
		t.Error("無効化後も 2FA 設定が残っている")
	}
}
