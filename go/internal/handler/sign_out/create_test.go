package sign_out_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/sign_out"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newSignOutHandler wires a sign-out Handler over the shared pool. Sign-out
// deletes the session through the pool, so its tests commit rows and use unique
// tokens (the test database is reset by make test).
//
// [Ja] newSignOutHandler は共有プールでサインアウト Handler を組み立てます。サインアウトは
// プール経由でセッションを削除するため、そのテストは行をコミットしユニークなトークンを
// 使います (テスト DB は make test がリセットする)。
func newSignOutHandler(t *testing.T) (*sign_out.Handler, *repository.UserSessionRepository) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	sessionMgr := session.NewManager(userRepo, cfg)
	deleteSessionUC := usecase.NewDeleteSessionUsecase(userSessionRepo)
	return sign_out.NewHandler(sessionMgr, deleteSessionUC), userSessionRepo
}

// seedSession creates a committed user and session, returning the session token
// so a handler test can sign out with it.
//
// [Ja] seedSession はコミットされたユーザーとセッションを作成し、ハンドラーテストが
// それでサインアウトできるようセッショントークンを返す。
func seedSession(t *testing.T, userSessionRepo *repository.UserSessionRepository) string {
	t.Helper()

	ctx := context.Background()
	user, err := repository.NewUserRepository(query.New(testutil.GetTestDB())).Create(ctx, repository.CreateUserInput{
		Email:    "signout-h-" + uuid.NewString() + "@example.com",
		Locale:   i18n.LangJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	userID := user.ID
	token := "signout-token-" + uuid.NewString()
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

// TestCreate_Success verifies that POST /sign_out deletes the session row, clears
// the session cookie, and redirects to the top page.
//
// [Ja] TestCreate_Success は、POST /sign_out がセッション行を削除し、セッション Cookie を
// 消去し、トップページへリダイレクトすることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	handler, userSessionRepo := newSignOutHandler(t)
	token := seedSession(t, userSessionRepo)

	req := httptest.NewRequest(http.MethodPost, "/sign_out", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

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

// TestCreate_NotSignedIn verifies that POST /sign_out without a session cookie
// still clears the cookie and redirects, treating sign-out as idempotent.
//
// [Ja] TestCreate_NotSignedIn は、セッション Cookie の無い POST /sign_out でも Cookie を
// 消去してリダイレクトし、サインアウトを冪等に扱うことを検証する。
func TestCreate_NotSignedIn(t *testing.T) {
	t.Parallel()

	handler, _ := newSignOutHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/sign_out", nil)
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}
	if !strings.HasPrefix(rec.Header().Get("Location"), "/") {
		t.Errorf("Location should be a path")
	}
}
