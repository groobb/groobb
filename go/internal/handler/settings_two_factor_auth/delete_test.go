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

// deleteTwoFactorAuthは現在のパスワードとTOTPコードをフォームデータ (空のものは
// 省く) として運ぶDELETE /settings/two_factor_authリクエストを組み立て、(RequireAuthが
// 置くように) contextのユーザーとロケールを載せる。フォームはリクエストがまだPOSTのうちに
// 解析し、その後でメソッドをDELETEに切り替える。これはメソッドオーバーライドミドルウェアの
// 再現で、GoのParseFormはPOST/PUT/PATCHのときだけボディを読むため、最初からDELETEとして
// 組み立てるとFormValueが空になる。
func deleteTwoFactorAuth(user *model.User, currentPassword, code string, locale model.Locale) *http.Request {
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

// TestDelete_SuccessWithPasswordは、正しい現在のパスワード付きの
// DELETE /settings/two_factor_authが2FAを無効化し (設定を削除する)、完了フラッシュを設定し、
// 設定ハブへリダイレクトすることを検証する。
func TestDelete_SuccessWithPassword(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, repo, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).WithEnabled(true).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(user.ID).Build()

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteTwoFactorAuth(user, testutil.DefaultBuilderPassword, "", model.LocaleJa))

	assertDisabled(t, rec, repo, user.ID)
}

// TestDelete_SuccessWithCodeは、正しい現在のTOTPコード (パスワードなし) 付きの
// DELETE /settings/two_factor_authが2FAを無効化し、パスワードを思い出せないユーザーも
// 認証アプリで無効化できることを検証する。
func TestDelete_SuccessWithCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, repo, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).WithEnabled(true).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteTwoFactorAuth(user, "", code, model.LocaleJa))

	assertDisabled(t, rec, repo, user.ID)
}

// TestDelete_ValidationErrorは、誤った現在のパスワード (コードなし) が無効化フォームを
// 422と資格情報誤りのメッセージで再描画し、2FAを有効なまま残すことを検証する。
func TestDelete_ValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, repo, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).WithEnabled(true).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(user.ID).Build()

	rec := httptest.NewRecorder()
	h.Delete(rec, deleteTwoFactorAuth(user, "wrongpassword", "", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "現在のパスワードか認証コードが正しくありません") {
		t.Error("資格情報誤りのエラーメッセージが描画されていない")
	}
	// 無効化フォームが再描画される (引き続きDELETE /settings/two_factor_authを動かす)。
	if !strings.Contains(body, `action="/settings/two_factor_auth"`) {
		t.Error("無効化フォームが再描画されていない")
	}

	// 2FAは有効なまま残る: 設定はまだ存在し有効である。
	stored, err := repo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("バリデーション失敗時に2FA設定が削除された")
	}
	if !stored.Enabled {
		t.Error("バリデーション失敗時に2FAが無効化された")
	}
}

// assertDisabledは無効化リクエストの成功時の共通期待を検証する。設定ハブへの303
// リダイレクト、無効化メッセージを運ぶ成功フラッシュ、そして2FA設定が削除されていること。
func assertDisabled(t *testing.T, rec *httptest.ResponseRecorder, repo *repository.UserTwoFactorAuthRepository, userID model.UserID) {
	t.Helper()

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/settings" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/settings")
	}

	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q、期待値 = %q", flash.Type, session.FlashSuccess)
	}
	want := i18n.T(i18n.SetLocale(context.Background(), model.LocaleJa), "flash_two_factor_auth_disabled")
	if flash.Message != want {
		t.Errorf("フラッシュのメッセージ = %q、期待値 = %q", flash.Message, want)
	}

	// 設定が削除され、2FAは無効になる。
	stored, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored != nil {
		t.Error("無効化後も2FA設定が残っている")
	}
}
