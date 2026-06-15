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

	// newHandler returns a handler that records whether it ran and the user
	// resolved from the context, so each case can assert what SetUser stored
	// and that the request was always passed through.
	//
	// [Ja] newHandler は実行されたかどうかと context から解決したユーザーを記録する
	// ハンドラーを返す。各ケースで SetUser が何を格納したか、そしてリクエストが常に
	// 素通しされたかを検証できる。
	newHandler := func(captured **model.User, called *bool) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*called = true
			*captured = middleware.UserFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})
	}

	t.Run("有効なセッション Cookie のとき現在のユーザーを context に格納する", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
		rec := httptest.NewRecorder()

		auth.SetUser(newHandler(&captured, &called)).ServeHTTP(rec, req)

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

		auth.SetUser(newHandler(&captured, &called)).ServeHTTP(rec, req)

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

		auth.SetUser(newHandler(&captured, &called)).ServeHTTP(rec, req)

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

		auth.SetUser(newHandler(&captured, &called)).ServeHTTP(rec, req)

		if !called {
			t.Fatal("次のハンドラーが呼ばれていない")
		}
		if captured != nil {
			t.Errorf("UserFromContext() = %v, want nil", captured)
		}
	})
}

func TestUserFromContext_NotSet(t *testing.T) {
	t.Parallel()

	if user := middleware.UserFromContext(context.Background()); user != nil {
		t.Errorf("UserFromContext() = %v, want nil", user)
	}
}
