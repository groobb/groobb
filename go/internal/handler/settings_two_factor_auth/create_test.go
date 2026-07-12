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

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/testutil"
)

// postCreate builds a POST /settings/two_factor_auth request carrying the TOTP code
// as form data, the user in the context (as RequireAuth would place it), and the
// locale set. It is a plain POST (no method override), so FormValue reads the body
// directly.
//
// [Ja] postCreate は TOTP コードをフォームデータとして運ぶ POST /settings/two_factor_auth
// リクエストを組み立て、(RequireAuth が置くように) context のユーザーとロケールを載せる。
// 素の POST (メソッドオーバーライドなし) のため、FormValue がボディを直接読む。
func postCreate(user *model.User, code, locale string) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// TestCreate_Success verifies that POST /settings/two_factor_auth with a correct
// TOTP code enables 2FA (the row becomes enabled with recovery codes) and renders
// the "enabled" page showing every recovery code once.
//
// [Ja] TestCreate_Success は、正しい TOTP コード付きの POST /settings/two_factor_auth が
// 2FA を有効化し (行が enabled になりリカバリーコードを持つ)、すべてのリカバリーコードを
// 一度だけ示す「有効化しました」ページを描画することを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	h, repo, user, tx := setupTwoFactorAuthHandler(t)
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(user.ID).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Create(rec, postCreate(user, code, i18n.LangJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	// The setting is now enabled with recovery codes.
	//
	// [Ja] 設定が有効になり、リカバリーコードを持つ。
	stored, err := repo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("有効化後の設定が取得できない")
	}
	if !stored.Enabled {
		t.Error("有効化後の Enabled = false, want true")
	}
	if len(stored.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("保存された len(RecoveryCodes) = %d, want %d", len(stored.RecoveryCodes), auth.RecoveryCodeCount)
	}

	// Every recovery code is rendered once so the user can record them.
	//
	// [Ja] 各リカバリーコードが一度描画され、ユーザーが控えられる。
	body := rec.Body.String()
	if !strings.Contains(body, "2 段階認証を有効にしました") {
		t.Error("有効化完了の見出しが描画されていない")
	}
	for _, recoveryCode := range stored.RecoveryCodes {
		if !strings.Contains(body, recoveryCode) {
			t.Errorf("リカバリーコード %q が描画されていない", recoveryCode)
		}
	}
}

// TestCreate_ValidationError verifies that a wrong TOTP code re-renders the
// enrollment form with 422 (QR and code field present) and the incorrect-code
// message, and leaves the setting not enabled.
//
// [Ja] TestCreate_ValidationError は、誤った TOTP コードが登録フォームを 422 (QR とコード
// フィールドを含む) とコード誤りのメッセージで再描画し、設定を未有効化のまま残すことを
// 検証する。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	h, repo, user, tx := setupTwoFactorAuthHandler(t)
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(user.ID).Build()

	// A well-formed code deliberately not equal to the current one.
	//
	// [Ja] 整った形式で、意図的に現在のコードと等しくない値。
	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	rec := httptest.NewRecorder()
	h.Create(rec, postCreate(user, wrongCode, i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	// The enrollment form is re-rendered (QR and code field present) with the error.
	//
	// [Ja] 登録フォームがエラー付きで再描画される (QR とコードフィールドを含む)。
	if !strings.Contains(body, "data:image/png;base64,") {
		t.Error("再描画時に QR コードが描画されていない")
	}
	if !strings.Contains(body, `name="code"`) {
		t.Error("再描画時にコード入力欄が描画されていない")
	}
	if !strings.Contains(body, "認証コードが正しくありません") {
		t.Error("コード誤りのエラーメッセージが描画されていない")
	}

	// The setting is left not enabled.
	//
	// [Ja] 設定は未有効化のまま残る。
	stored, err := repo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("登録行が消えている")
	}
	if stored.Enabled {
		t.Error("誤ったコードで Enabled = true になった (有効化されるべきでない)")
	}
}

// TestCreate_AlreadyEnabled verifies that re-submitting the enable form after 2FA is
// already on is not a dead end: the validator reports the setup as gone, and the
// re-render resolves the already-enabled setting and shows the disable confirmation
// form (422) instead of re-enrolling. This covers the reload-after-success path end
// to end at the handler level.
//
// [Ja] TestCreate_AlreadyEnabled は、2FA が既に有効な後に有効化フォームを再送しても
// 行き止まりにならないことを検証する。validator が設定は失われたと報告し、再描画が
// 既に有効な設定を解決して、再登録せず無効化の確認フォーム (422) を表示する。成功後の
// リロード経路をハンドラーレベルで端から端まで網羅する。
func TestCreate_AlreadyEnabled(t *testing.T) {
	t.Parallel()

	h, _, user, tx := setupTwoFactorAuthHandler(t)
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(user.ID).WithEnabled(true).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Create(rec, postCreate(user, code, i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	// The disable confirmation form is shown (heading + DELETE form) instead of
	// re-enrolling; no QR is rendered.
	//
	// [Ja] 再登録せず無効化の確認フォーム (見出し + DELETE フォーム) が表示される。QR は
	// 描画されない。
	if !strings.Contains(body, "2 段階認証の無効化") {
		t.Error("既に有効な状態で有効化を再送したのに無効化フォームが描画されていない")
	}
	if !strings.Contains(body, `value="DELETE"`) {
		t.Error("無効化フォーム (DELETE) が描画されていない")
	}
	if strings.Contains(body, "data:image/png;base64,") {
		t.Error("既に有効なのに登録用 QR が描画されている")
	}
}
