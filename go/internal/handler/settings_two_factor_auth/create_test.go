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

// postCreateはTOTPコードをフォームデータとして運ぶPOST /settings/two_factor_auth
// リクエストを組み立て、(RequireAuthが置くように) contextのユーザーとロケールを載せる。
// 素のPOST (メソッドオーバーライドなし) のため、FormValueがボディを直接読む。
func postCreate(user *model.User, code string, locale model.Locale) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// TestCreate_Successは、正しいTOTPコード付きのPOST /settings/two_factor_authが
// 2FAを有効化し (行がenabledになりリカバリーコードを持つ)、すべてのリカバリーコードを
// 一度だけ示す「有効化しました」ページを描画することを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, repo, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Create(rec, postCreate(user, code, model.LocaleJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	// 設定が有効になり、リカバリーコードを持つ。
	stored, err := repo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("有効化後の設定が取得できない")
	}
	if !stored.Enabled {
		t.Error("有効化後のEnabled = false、期待値 = true")
	}
	if len(stored.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("保存されたlen(RecoveryCodes) = %d、期待値 = %d", len(stored.RecoveryCodes), auth.RecoveryCodeCount)
	}

	// 各リカバリーコードが一度描画され、ユーザーが控えられる。
	body := rec.Body.String()
	if !strings.Contains(body, "2段階認証を有効にしました") {
		t.Error("有効化完了の見出しが描画されていない")
	}
	if !strings.Contains(body, `aria-label="グローバルナビゲーション"`) {
		t.Error("サインイン済みページ共通ヘッダーのナビゲーションが描画されていない")
	}
	if !strings.Contains(body, `href="/home"`) {
		t.Error("サインイン済みページ共通ヘッダーのホームリンクが描画されていない")
	}
	for _, recoveryCode := range stored.RecoveryCodes {
		if !strings.Contains(body, recoveryCode) {
			t.Errorf("リカバリーコード %q が描画されていない", recoveryCode)
		}
	}
}

// TestCreate_ValidationErrorは、誤ったTOTPコードが登録フォームを422 (QRとコード
// フィールドを含む) とコード誤りのメッセージで再描画し、設定を未有効化のまま残すことを
// 検証する。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, repo, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).Build()

	// 整った形式で、意図的に現在のコードと等しくない値。
	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	rec := httptest.NewRecorder()
	h.Create(rec, postCreate(user, wrongCode, model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	// 登録フォームがエラー付きで再描画される (QRとコードフィールドを含む)。
	if !strings.Contains(body, "data:image/png;base64,") {
		t.Error("再描画時にQRコードが描画されていない")
	}
	if !strings.Contains(body, `name="code"`) {
		t.Error("再描画時にコード入力欄が描画されていない")
	}
	if !strings.Contains(body, "認証コードが正しくありません") {
		t.Error("コード誤りのエラーメッセージが描画されていない")
	}

	// 設定は未有効化のまま残る。
	stored, err := repo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("登録行が消えている")
	}
	if stored.Enabled {
		t.Error("誤ったコードでEnabled = trueになった (有効化されるべきでない)")
	}
}

// TestCreate_AlreadyEnabledは、2FAが既に有効な後に有効化フォームを再送しても
// 行き止まりにならないことを検証する。validatorが設定は失われたと報告し、再描画が
// 既に有効な設定を解決して、再登録せず無効化の確認フォーム (422) を表示する。成功後の
// リロード経路をハンドラーレベルで端から端まで網羅する。
func TestCreate_AlreadyEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, _, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).WithEnabled(true).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Create(rec, postCreate(user, code, model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	// 再登録せず無効化の確認フォーム (見出し + DELETEフォーム) が表示される。QRは
	// 描画されない。
	if !strings.Contains(body, "2段階認証の無効化") {
		t.Error("既に有効な状態で有効化を再送したのに無効化フォームが描画されていない")
	}
	if !strings.Contains(body, `value="DELETE"`) {
		t.Error("無効化フォーム (DELETE) が描画されていない")
	}
	if strings.Contains(body, "data:image/png;base64,") {
		t.Error("既に有効なのに登録用QRが描画されている")
	}
}
