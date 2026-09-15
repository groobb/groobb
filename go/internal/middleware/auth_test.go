package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

func TestAuth_SetUser(t *testing.T) {
	t.Parallel()

	auth, userID := setupAuthTest(t)

	t.Run("有効なセッションCookieのとき現在のユーザーをcontextに格納する", func(t *testing.T) {
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
			t.Fatal("UserFromContext() = nil、期待値はユーザー")
		}
		if captured.ID != userID {
			t.Errorf("user.ID = %v、期待値 = %v", captured.ID, userID)
		}
	})

	t.Run("Cookieが無いときはユーザーを格納せず素通しする", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		auth.SetUser(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if !called {
			t.Fatal("次のハンドラーが呼ばれていない")
		}
		if captured != nil {
			t.Errorf("UserFromContext() = %v、期待値 = nil", captured)
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
			t.Errorf("UserFromContext() = %v、期待値 = nil", captured)
		}
	})

	t.Run("ユーザー解決が失敗してもユーザーを格納せず素通しする", func(t *testing.T) {
		var captured *model.User
		var called bool

		// contextを先にキャンセルしてセッション解決をエラーにし、warnして
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
			t.Errorf("UserFromContext() = %v、期待値 = nil", captured)
		}
	})
}

func TestAuth_RequireAuth(t *testing.T) {
	t.Parallel()

	auth, userID := setupAuthTest(t)

	t.Run("サインイン済みのとき素通ししてユーザーをcontextに格納する", func(t *testing.T) {
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
			t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
		}
		if captured == nil {
			t.Fatal("UserFromContext() = nil、期待値はユーザー")
		}
		if captured.ID != userID {
			t.Errorf("user.ID = %v、期待値 = %v", captured.ID, userID)
		}
		assertPrivateNoCache(t, rec)
	})

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run("未サインインの"+method+"は元のURLをreturn_toに載せて /sign_inへリダイレクトする", func(t *testing.T) {
			var captured *model.User
			var called bool

			req := httptest.NewRequest(method, "/settings?from=home", nil)
			rec := httptest.NewRecorder()

			auth.RequireAuth(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

			if called {
				t.Fatal("未サインインなのに次のハンドラーが呼ばれた")
			}
			if rec.Code != http.StatusSeeOther {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			want := "/sign_in?return_to=%2Fsettings%3Ffrom%3Dhome"
			if loc := rec.Header().Get("Location"); loc != want {
				t.Errorf("Location = %q、期待値 = %q", loc, want)
			}
			assertPrivateNoCache(t, rec)
		})
	}

	// POSTの宛先を後からGETで開いても訪問者が求めた場所ではないため、安全でない
	// メソッドは素のサインインパスにフォールバックする。
	t.Run("未サインインのPOSTはreturn_toを載せずに /sign_inへリダイレクトする", func(t *testing.T) {
		var captured *model.User
		var called bool

		req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", nil)
		rec := httptest.NewRecorder()

		auth.RequireAuth(newRecordingHandler(&captured, &called)).ServeHTTP(rec, req)

		if called {
			t.Fatal("未サインインなのに次のハンドラーが呼ばれた")
		}
		if rec.Code != http.StatusSeeOther {
			t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
		}
		if loc := rec.Header().Get("Location"); loc != "/sign_in" {
			t.Errorf("Location = %q、期待値 = %q", loc, "/sign_in")
		}
		assertPrivateNoCache(t, rec)
	})

	t.Run("ユーザー解決が失敗したとき500を返す", func(t *testing.T) {
		var captured *model.User
		var called bool

		// contextを先にキャンセルしてセッション解決をエラーにし、500で応答する
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
			t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
		}
		assertPrivateNoCache(t, rec)
	})

	// 方針はハンドラーが走る前に設定するため、その後ハンドラーが何を書いても残る
	// 必要がある。保護されたハンドラーは描画したページではなくhttp.Errorや
	// http.Redirectで応答することがあり、どちらのヘルパーも出ていく際に自前でヘッダーを
	// 書き換える。
	for _, tt := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "http.Errorで404を書いても",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
		},
		{
			name: "http.Redirectで301を書いても",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/settings", http.StatusMovedPermanently)
			},
		},
	} {
		t.Run("ハンドラーが"+tt.name+"キャッシュ方針が残る", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/settings/email/edit", nil)
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})
			rec := httptest.NewRecorder()

			auth.RequireAuth(tt.handler).ServeHTTP(rec, req)

			assertPrivateNoCache(t, rec)
		})
	}

	// settings_two_factor_authは平文のsecretとリカバリーコードを表示するページの
	// ため、値をno-storeで置き換える。ここで設定する既定が、それらのレスポンスに残る
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
			t.Errorf("Cache-Control = %q、期待値 = %q", got, "no-store")
		}
	})
}

func TestUserFromContext_NotSet(t *testing.T) {
	t.Parallel()

	if user := middleware.UserFromContext(context.Background()); user != nil {
		t.Errorf("UserFromContext() = %v、期待値 = nil", user)
	}
}

// TestUserIDFromContextはUseCaseへ渡す任意のidが、匿名リクエストではnil、
// サインイン済みリクエストでは現在のユーザーのidになることを検証する。
func TestUserIDFromContext(t *testing.T) {
	t.Parallel()

	userID := model.UserID(42)
	tests := []struct {
		name string
		ctx  context.Context
		want *model.UserID
	}{
		{
			name: "ユーザーが格納されていないときはnilを返す",
			ctx:  context.Background(),
		},
		{
			name: "ユーザーが格納されているときはそのidを返す",
			ctx: middleware.SetUserToContext(
				context.Background(),
				&model.User{ID: userID},
			),
			want: &userID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := middleware.UserIDFromContext(tt.ctx)
			if tt.want == nil {
				if got != nil {
					t.Errorf("UserIDFromContext() = %v、期待値 = nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("UserIDFromContext() = nil、期待値はユーザーID")
			}
			if *got != *tt.want {
				t.Errorf("UserIDFromContext() = %v、期待値 = %v", *got, *tt.want)
			}
		})
	}
}

// setupAuthTestはテスト専用のデータベースに紐づくAuthミドルウェアを
// 組み立て、"valid-token" のセッションを持つユーザーを1人シードして、その
// ミドルウェアとシードしたユーザーのidを返す。SetUserとRequireAuthは同じ方法で
// 現在のユーザーを解決するため、両者でこのフィクスチャを共有する。
func setupAuthTest(t *testing.T) (*middleware.Auth, model.UserID) {
	t.Helper()

	db := testutil.SetupDB(t)
	cfg := &config.Config{Env: "test"}
	userRepo := repository.NewUserRepository(db)
	mgr := session.NewManager(userRepo, cfg)
	auth := middleware.NewAuth(mgr)

	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserSessionBuilder(t, db).
		WithUserID(userID).
		WithToken("valid-token").
		Build()

	return auth, userID
}

// assertPrivateNoCacheはRequireAuthが応答するすべてに付けるキャッシュ方針が
// レスポンスに載っていなければテストを失敗させる。レスポンスを返す各ケースで検証するのは、
// ページを描画する経路だけでなくミドルウェアから出るすべての経路で成り立って初めて、
// 保護されたルートの方針が保証されるためである。
func assertPrivateNoCache(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-cache")
	}
}

// newRecordingHandlerは実行されたかどうかとリクエストcontextから解決した
// ユーザーを記録するハンドラーを返す。各ケースでミドルウェアがリクエストを素通し
// させたか、そして何を格納したかを検証できる。
func newRecordingHandler(captured **model.User, called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*captured = middleware.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}
