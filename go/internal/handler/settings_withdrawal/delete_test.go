package settings_withdrawal_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/settings_withdrawal"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSettingsWithdrawalHandlerはテスト用データベースのリポジトリで
// settings_withdrawal Handlerを組み立てます。ハンドラーテストがDeleteAccountUsecase
// と、それが開くトランザクションを実DBに対して駆動できるようにするためです。
func newSettingsWithdrawalHandler(t *testing.T, db *database.DB) *settings_withdrawal.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	userRoleRepo := repository.NewUserRoleRepository(db)

	sessionMgr := session.NewManager(userRepo, cfg)
	flashMgr := session.NewFlashManager(cfg)
	deleteAccountUC := usecase.NewDeleteAccountUsecase(
		db.Writer,
		validator.NewSettingsWithdrawalDeleteValidator(userPasswordRepo),
		userRepo,
		userSessionRepo,
		roleRepo,
		userRoleRepo,
	)
	return settings_withdrawal.NewHandler(cfg, sessionMgr, flashMgr, deleteAccountUC)
}

// seedWithdrawalUserはパスワード "password123" と有効なセッションを1つ持つ
// コミット済みユーザーを作成し、ユーザーモデル (テストがRequireAuthのようにcontextに
// 載せられるよう) とセッショントークン (リクエストが一致するセッションCookieを運べるよう)
// を返します。
func seedWithdrawalUser(t *testing.T, db *database.DB) (*model.User, string) {
	t.Helper()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    "wd-h@example.com",
		Atname:   testutil.UniqueAtname(db),
		Locale:   model.LocaleJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}
	digest, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("テスト用パスワードの作成に失敗: %v", err)
	}
	token := "wd-h-token"
	if _, err := userSessionRepo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    user.ID,
		Token:     token,
		IPAddress: "203.0.113.7",
		UserAgent: "test-agent",
	}); err != nil {
		t.Fatalf("テスト用セッションの作成に失敗: %v", err)
	}
	return user, token
}

// deleteWithdrawalは現在のパスワードをフォームデータとして運ぶ
// DELETE /settings/withdrawalリクエストを組み立て、tokenのセッションCookie、
// (RequireAuthが置くように) contextのユーザー、そして設定したロケールを載せます。
// フォームはリクエストがまだPOSTのうちに解析し、その後でメソッドをDELETEに切り替えます。
// これは本番のメソッドオーバーライドミドルウェアの挙動を再現したものです。GoのParseFormは
// POST/PUT/PATCHのときだけボディを読むため、最初からDELETEとして組み立てたリクエストでは
// ハンドラーのFormValueが空になってしまいます。
func deleteWithdrawal(user *model.User, currentPassword, token string, locale model.Locale) *http.Request {
	form := url.Values{"current_password": {currentPassword}}
	req := httptest.NewRequest(http.MethodPost, "/settings/withdrawal", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		panic(err)
	}
	req.Method = http.MethodDelete
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// findCookieはレスポンスから指定名のCookieを返す。無ければnil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// decodeFlashはレスポンスのフラッシュCookieを読み取ってデコードする。
// FlashManagerが書き込むbase64エンコードされたJSONと対になる。Cookieが無い、
// または壊れている場合はテストを失敗させる。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	c := findCookie(rec, session.FlashCookieName)
	if c == nil {
		t.Fatal("フラッシュCookieが設定されていない")
	}
	data, err := base64.StdEncoding.DecodeString(c.Value)
	if err != nil {
		t.Fatalf("フラッシュCookieのbase64デコードに失敗: %v", err)
	}
	var flash session.FlashMessage
	if err := json.Unmarshal(data, &flash); err != nil {
		t.Fatalf("フラッシュCookieのJSONデコードに失敗: %v", err)
	}
	return &flash
}

// TestDelete_Successは、正しい現在のパスワード付きのDELETE /settings/withdrawalが
// アカウントを退会させ (ユーザーを論理削除・匿名化し、そのセッションを削除する)、
// セッションCookieを消去し、完了フラッシュを設定し、トップページへリダイレクトすることを
// 検証する。
func TestDelete_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSettingsWithdrawalHandler(t, db)
	user, token := seedWithdrawalUser(t, db)

	rec := httptest.NewRecorder()
	handler.Delete(rec, deleteWithdrawal(user, "password123", token, model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/")
	}

	// セッションCookieが消去される (MaxAge < 0の同名Cookie)。
	if c := findCookie(rec, session.CookieName); c == nil || c.MaxAge >= 0 {
		t.Error("セッションCookieが消去されていない")
	}

	// トップページが「退会しました」toastを描画するよう成功フラッシュが設定される。
	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q、期待値 = %q", flash.Type, session.FlashSuccess)
	}
	if want := i18n.T(i18n.SetLocale(context.Background(), model.LocaleJa), "flash_account_withdrawn"); flash.Message != want {
		t.Errorf("フラッシュのメッセージ = %q、期待値 = %q", flash.Message, want)
	}

	// ユーザー行が論理削除される。行は (deleted_atで絞るルックアップではなく) 直接
	// クエリするため、論理削除されたユーザーも観測できる。
	var deletedAt *time.Time
	if err := db.Reader.QueryRowContext(context.Background(),
		`SELECT deleted_at FROM users WHERE id = ?`, int64(user.ID),
	).Scan(&deletedAt); err != nil {
		t.Fatalf("退会後のユーザー行の取得に失敗: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_atがセットされていない (論理削除されていない)")
	}

	// セッション行が消えている (UseCaseが全端末をサインアウトさせた)。
	if got := countUserSessions(t, db, user.ID); got != 0 {
		t.Errorf("退会後のセッション数 = %d、期待値 = 0", got)
	}
}

// TestDelete_ValidationErrorは、誤った現在のパスワードが確認フォームを422と
// パスワード誤りのメッセージで再描画し、アカウントを完全に無傷のまま (論理削除されず、
// セッションも残ったまま) にすることを検証する。
func TestDelete_ValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSettingsWithdrawalHandler(t, db)
	user, _ := seedWithdrawalUser(t, db)

	rec := httptest.NewRecorder()
	handler.Delete(rec, deleteWithdrawal(user, "wrongpassword", "wd-h-token-unused", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "現在のパスワードが正しくありません") {
		t.Error("現在パスワード誤りのエラーメッセージが描画されていない")
	}
	// スクリーンリーダーがメッセージを読み上げ、入力欄に関連付けられるよう、
	// アクセシブルなエラーマークアップがメッセージに伴っていること。
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄にaria-invalid='true' が無い")
	}
	// 確認フォームが再描画される (引き続きDELETE /settings/withdrawalを動かす)。
	if !strings.Contains(body, `action="/settings/withdrawal"`) {
		t.Error("退会確認フォームが再描画されていない")
	}

	// アカウントは無傷: 論理削除されず、セッションも残る。
	var deletedAt *time.Time
	if err := db.Reader.QueryRowContext(context.Background(),
		`SELECT deleted_at FROM users WHERE id = ?`, int64(user.ID),
	).Scan(&deletedAt); err != nil {
		t.Fatalf("ユーザー行の取得に失敗: %v", err)
	}
	if deletedAt != nil {
		t.Error("バリデーション失敗時にユーザーが論理削除された")
	}
	if got := countUserSessions(t, db, user.ID); got != 1 {
		t.Errorf("バリデーション失敗時のセッション数 = %d、期待値 = 1 (削除されるべきでない)", got)
	}
}

// countUserSessionsは指定ユーザーがまだ所有するセッション数を返す。退会が
// それらを消したこと (または拒否された退会がそれらを残したこと) を検証するために使う。
func countUserSessions(t *testing.T, db *database.DB, userID model.UserID) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(),
		`SELECT count(*) FROM user_sessions WHERE user_id = ?`, int64(userID),
	).Scan(&count); err != nil {
		t.Fatalf("セッション件数の取得に失敗: %v", err)
	}
	return count
}
