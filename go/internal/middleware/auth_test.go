package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

func TestAuth_SetUser(t *testing.T) {
	t.Parallel()

	auth, userID := setupAuthTest(t)

	t.Run("有効なセッション Cookie のとき現在のユーザーを context に格納する", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
		rec := httptest.NewRecorder()

		auth.SetUser(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if !called {
			t.Fatal("次のハンドラーが呼ばれていない")
		}
		if captured == nil {
			t.Fatal("UserFromContext() = nil, want user")
		}
		if captured.ID != userID {
			t.Errorf("user.ID = %v, want %v", captured.ID, userID)
		}
	})

	t.Run("Cookie が無いときはユーザーを格納せず素通しする", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		auth.SetUser(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if !called {
			t.Fatal("次のハンドラーが呼ばれていない")
		}
		if captured != nil {
			t.Errorf("UserFromContext() = %v, want nil", captured)
		}
	})

	t.Run("未知のトークンのときはユーザーを格納せず素通しする", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "unknown-token"})
		rec := httptest.NewRecorder()

		auth.SetUser(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if !called {
			t.Fatal("次のハンドラーが呼ばれていない")
		}
		if captured != nil {
			t.Errorf("UserFromContext() = %v, want nil", captured)
		}
	})

	t.Run("ユーザー解決が失敗してもユーザーを格納せず素通しする", func(t *testing.T) {
		var captured *model.User
		var called bool

		// Cancel the context up front so the session lookup errors out, exercising
		// the warn-and-pass-through branch.
		//
		// [Ja] context を先にキャンセルしてセッション解決をエラーにし、warn して
		// 素通しする分岐を通す。
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
		rec := httptest.NewRecorder()

		auth.SetUser(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if !called {
			t.Fatal("次のハンドラーが呼ばれていない")
		}
		if captured != nil {
			t.Errorf("UserFromContext() = %v, want nil", captured)
		}
	})
}

func TestAuth_RequireAuth(t *testing.T) {
	t.Parallel()

	auth, userID := setupAuthTest(t)

	t.Run("サインイン済みのとき素通ししてユーザーを context に格納する", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodGet, "/home", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
		rec := httptest.NewRecorder()

		auth.RequireAuth(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if !called {
			t.Fatal("次のハンドラーが呼ばれていない")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if captured == nil {
			t.Fatal("UserFromContext() = nil, want user")
		}
		if captured.ID != userID {
			t.Errorf("user.ID = %v, want %v", captured.ID, userID)
		}
	})

	t.Run("未サインインのとき /sign_in へリダイレクトする", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodGet, "/home", nil)
		rec := httptest.NewRecorder()

		auth.RequireAuth(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if called {
			t.Fatal("未サインインなのに次のハンドラーが呼ばれた")
		}
		if rec.Code != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
		}
		if loc := rec.Header().Get("Location"); loc != "/sign_in" {
			t.Errorf("Location = %q, want %q", loc, "/sign_in")
		}
	})

	t.Run("ユーザー解決が失敗したとき 500 を返す", func(t *testing.T) {
		var captured *model.User
		var called bool

		// Cancel the context up front so the session lookup errors out, exercising
		// the fatal branch that answers with 500.
		//
		// [Ja] context を先にキャンセルしてセッション解決をエラーにし、500 で応答する
		// 致命的分岐を通す。
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest(http.MethodGet, "/home", nil).WithContext(ctx)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
		rec := httptest.NewRecorder()

		auth.RequireAuth(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if called {
			t.Fatal("解決失敗なのに次のハンドラーが呼ばれた")
		}
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestUserFromContext_NotSet(t *testing.T) {
	t.Parallel()

	if user := middleware.UserFromContext(context.Background()); user != nil {
		t.Errorf("UserFromContext() = %v, want nil", user)
	}
}

// setupAuthTest builds an Auth middleware backed by a fresh test transaction and
// seeds one user with a "valid-token" session, returning the middleware and the
// seeded user's id. SetUser and RequireAuth resolve the current user the same
// way, so both share this fixture.
//
// [Ja] setupAuthTest は新しいテスト用トランザクションに紐づく Auth ミドルウェアを
// 組み立て、"valid-token" のセッションを持つユーザーを 1 人シードして、その
// ミドルウェアとシードしたユーザーの id を返す。SetUser と RequireAuth は同じ方法で
// 現在のユーザーを解決するため、両者でこのフィクスチャを共有する。
func setupAuthTest(t *testing.T) (*middleware.Auth, model.UserID) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	cfg := &config.Config{Env: "test"}
	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	mgr := session.NewManager(userRepo, cfg)
	auth := middleware.NewAuth(mgr)

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserSessionBuilder(t, tx).
		WithUserID(userID).
		WithToken("valid-token").
		Build()

	return auth, userID
}

// newRecordingHandler returns a handler that records whether it ran and the user
// resolved from the request context, so each case can assert whether the
// middleware passed the request through and what it stored.
//
// [Ja] newRecordingHandler は実行されたかどうかとリクエスト context から解決した
// ユーザーを記録するハンドラーを返す。各ケースでミドルウェアがリクエストを素通し
// させたか、そして何を格納したかを検証できる。
func newRecordingHandler(captured **model.User, called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*captured = middleware.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}
