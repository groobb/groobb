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

// newUserSessionHandler wires a user session Handler over the test database's
// repositories, so a sign-out test deletes a real session row.
//
// [Ja] newUserSessionHandler はテスト用データベースのリポジトリでユーザーセッション
// Handler を組み立てます。サインアウトのテストが実在のセッション行を削除できるように
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

// seedSession creates a committed user and session, returning the session token
// so a handler test can sign out with it.
//
// [Ja] seedSession はコミットされたユーザーとセッションを作成し、ハンドラーテストが
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

// findCookie returns the cookie with the given name from the response, or nil.
//
// [Ja] findCookie はレスポンスから指定名の Cookie を返す。無ければ nil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// decodeFlash reads and decodes the flash cookie from the response, mirroring the
// base64-encoded JSON that FlashManager writes. It fails the test if the cookie is
// missing or malformed.
//
// [Ja] decodeFlash はレスポンスのフラッシュ Cookie を読み取ってデコードする。
// FlashManager が書き込む base64 エンコードされた JSON と対になる。Cookie が無い、
// または壊れている場合はテストを失敗させる。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	c := findCookie(rec, session.FlashCookieName)
	if c == nil {
		t.Fatal("フラッシュ Cookie が設定されていない")
	}
	data, err := base64.StdEncoding.DecodeString(c.Value)
	if err != nil {
		t.Fatalf("フラッシュ Cookie の base64 デコードに失敗: %v", err)
	}
	var flash session.FlashMessage
	if err := json.Unmarshal(data, &flash); err != nil {
		t.Fatalf("フラッシュ Cookie の JSON デコードに失敗: %v", err)
	}
	return &flash
}

// TestDelete_Success verifies that DELETE /user_session deletes the session row,
// clears the session cookie, and redirects to the top page.
//
// [Ja] TestDelete_Success は、DELETE /user_session がセッション行を削除し、セッション
// Cookie を消去し、トップページへリダイレクトすることを検証する。
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
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}

	// The session cookie is cleared (a matching cookie with MaxAge < 0).
	//
	// [Ja] セッション Cookie が消去される (MaxAge < 0 の同名 Cookie)。
	if c := findCookie(rec, session.CookieName); c == nil || c.MaxAge >= 0 {
		t.Error("セッション Cookie が消去されていない")
	}

	// A success flash is set so the redirect target renders the "signed out" toast.
	//
	// [Ja] リダイレクト先が「ログアウトしました」toast を描画するよう成功フラッシュが設定される。
	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("flash type = %q, want %q", flash.Type, session.FlashSuccess)
	}
	if want := i18n.T(req.Context(), "flash_sign_out_success"); flash.Message != want {
		t.Errorf("flash message = %q, want %q", flash.Message, want)
	}

	// The session row is gone, so the token no longer resolves.
	//
	// [Ja] セッション行が消え、token がもう解決しない。
	s, err := userSessionRepo.FindByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("FindByToken() error = %v", err)
	}
	if s != nil {
		t.Error("サインアウト後もセッション行が残っている")
	}
}

// TestDelete_NotSignedIn verifies that DELETE /user_session without a session
// cookie still clears the cookie and redirects, treating sign-out as idempotent.
//
// [Ja] TestDelete_NotSignedIn は、セッション Cookie の無い DELETE /user_session でも
// Cookie を消去してリダイレクトし、サインアウトを冪等に扱うことを検証する。
func TestDelete_NotSignedIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newUserSessionHandler(t, db)

	req := httptest.NewRequest(http.MethodDelete, "/user_session", nil)
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}
}
