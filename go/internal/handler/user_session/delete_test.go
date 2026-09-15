package user_session_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/user_session"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newUserSessionHandlerはテスト用データベースのリポジトリでユーザーセッション
// Handlerを組み立てます。サインアウトのテストが実在のセッション行を削除できるように
// するためです。
func newUserSessionHandler(t *testing.T, db *database.DB) (*user_session.Handler, *repository.UserSessionRepository) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	userRepo := repository.NewUserRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	sessionMgr := session.NewManager(userRepo, cfg)
	flashMgr := session.NewFlashManager(cfg)
	deleteSessionUC := usecase.NewDeleteSessionUsecase(userSessionRepo)
	return user_session.NewHandler(sessionMgr, flashMgr, deleteSessionUC), userSessionRepo
}

// seedSessionはコミットされたユーザーとセッションを作成し、ハンドラーテストが
// それでサインアウトできるようセッショントークンを返す。
func seedSession(t *testing.T, db *database.DB, userSessionRepo *repository.UserSessionRepository) string {
	t.Helper()

	ctx := context.Background()
	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    "signout-h@example.com",
		Atname:   testutil.UniqueAtname(db),
		Locale:   model.LocaleJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	userID := user.ID
	token := "signout-token"
	if _, err := userSessionRepo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     token,
		IPAddress: "203.0.113.7",
		UserAgent: "test-agent",
	}); err != nil {
		t.Fatalf("セッションの作成に失敗: %v", err)
	}
	return token
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

// TestDelete_Successは、DELETE /user_sessionがセッション行を削除し、セッション
// Cookieを消去し、トップページへリダイレクトすることを検証する。
func TestDelete_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, userSessionRepo := newUserSessionHandler(t, db)
	token := seedSession(t, db, userSessionRepo)

	req := httptest.NewRequest(http.MethodDelete, "/user_session", nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

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

	// リダイレクト先が「ログアウトしました」toastを描画するよう成功フラッシュが設定される。
	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q、期待値 = %q", flash.Type, session.FlashSuccess)
	}
	if want := i18n.T(req.Context(), "flash_sign_out_success"); flash.Message != want {
		t.Errorf("フラッシュのメッセージ = %q、期待値 = %q", flash.Message, want)
	}

	// セッション行が消え、tokenがもう解決しない。
	s, err := userSessionRepo.FindByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("FindByToken()のエラー = %v", err)
	}
	if s != nil {
		t.Error("サインアウト後もセッション行が残っている")
	}
}

// TestDelete_NotSignedInは、セッションCookieの無いDELETE /user_sessionでも
// Cookieを消去してリダイレクトし、サインアウトを冪等に扱うことを検証する。
func TestDelete_NotSignedIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newUserSessionHandler(t, db)

	req := httptest.NewRequest(http.MethodDelete, "/user_session", nil)
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/")
	}
}
