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
		assertPrivateNoCache(t, rec)
	})

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run("未サインインの "+method+" は元の URL を return_to に載せて /sign_in へリダイレクトする", func(t *testing.T) {
			var captured *model.User
			var called bool

			req := httptest.NewRequest(method, "/settings?from=home", nil)
			rec := httptest.NewRecorder()

			auth.RequireAuth(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

			if called {
				t.Fatal("未サインインなのに次のハンドラーが呼ばれた")
			}
			if rec.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			want := "/sign_in?return_to=%2Fsettings%3Ffrom%3Dhome"
			if loc := rec.Header().Get("Location"); loc != want {
				t.Errorf("Location = %q, want %q", loc, want)
			}
			assertPrivateNoCache(t, rec)
		})
	}

	// A POST target replayed as a GET landing page is not where the visitor asked
	// to go, so an unsafe method falls back to the bare sign-in path.
	//
	// [Ja] POST の宛先を後から GET で開いても訪問者が求めた場所ではないため、安全でない
	// メソッドは素のサインインパスにフォールバックする。
	t.Run("未サインインの POST は return_to を載せずに /sign_in へリダイレクトする", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", nil)
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
		assertPrivateNoCache(t, rec)
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
		assertPrivateNoCache(t, rec)
	})

	// The policy is set before the handler runs, so it has to survive whatever the
	// handler writes afterwards. A guarded handler may answer with http.Error or
	// http.Redirect instead of a rendered page, and both helpers rewrite headers of
	// their own on the way out.
	//
	// [Ja] 方針はハンドラーが走る前に設定するため、その後ハンドラーが何を書いても残る
	// 必要がある。保護されたハンドラーは描画したページではなく http.Error や
	// http.Redirect で応答することがあり、どちらのヘルパーも出ていく際に自前でヘッダーを
	// 書き換える。
	for _, tt := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "http.Error で 404 を書いても",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
		},
		{
			name: "http.Redirect で 301 を書いても",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/settings", http.StatusMovedPermanently)
			},
		},
	} {
		t.Run("ハンドラーが "+tt.name+"キャッシュ方針が残る", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/settings/email/edit", nil)
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
			rec := httptest.NewRecorder()

			auth.RequireAuth(tt.handler).ServeHTTP(rec, req)

			assertPrivateNoCache(t, rec)
		})
	}

	// settings_two_factor_auth replaces the value with no-store because its pages
	// show a plaintext secret and recovery codes. The default set here must not be
	// what those responses end up carrying.
	//
	// [Ja] settings_two_factor_auth は平文の secret とリカバリーコードを表示するページの
	// ため、値を no-store で置き換える。ここで設定する既定が、それらのレスポンスに残る
	// 値であってはならない。
	t.Run("ハンドラーがより厳しいキャッシュ方針で上書きできる", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth/new", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
		rec := httptest.NewRecorder()

		auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rec, req)

		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want %q", got, "no-store")
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

// assertPrivateNoCache fails the test unless the response carries the cache
// policy RequireAuth puts on everything it answers. Each case that produces a
// response asserts it, because the policy is only guaranteed for a guarded route
// if it holds on every path out of the middleware, not just the one that renders
// a page.
//
// [Ja] assertPrivateNoCache は RequireAuth が応答するすべてに付けるキャッシュ方針が
// レスポンスに載っていなければテストを失敗させる。レスポンスを返す各ケースで検証するのは、
// ページを描画する経路だけでなくミドルウェアから出るすべての経路で成り立って初めて、
// 保護されたルートの方針が保証されるためである。
func assertPrivateNoCache(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-cache")
	}
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
