package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/middleware"
)

// newCSRFRecorder returns a handler that records whether it ran and the CSRF
// token visible in its context, so each case can assert what Middleware passed
// through and stored.
//
// [Ja] newCSRFRecorder は実行されたかどうかと context から見える CSRF トークンを
// 記録するハンドラーを返す。各ケースで Middleware が何を素通し・格納したかを
// 検証できる。
func newCSRFRecorder(called *bool, ctxToken *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*ctxToken = middleware.CSRFTokenFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestCSRF_IssuesTokenOnSafeMethods(t *testing.T) {
	t.Parallel()

	c := middleware.NewCSRF(&config.Config{Env: "test"})

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			var called bool
			var ctxToken string

			req := httptest.NewRequest(method, "/", nil)
			rec := httptest.NewRecorder()

			c.Middleware(newCSRFRecorder(&called, &ctxToken)).ServeHTTP(rec, req)

			if !called {
				t.Fatal("次のハンドラーが呼ばれていない")
			}
			cookie := findCookie(rec, middleware.CSRFCookieName)
			if cookie == nil || cookie.Value == "" {
				t.Fatalf("CSRF Cookie %q が発行されていない", middleware.CSRFCookieName)
			}
			if ctxToken != cookie.Value {
				t.Errorf("context のトークン = %q, want %q (発行した Cookie と一致)", ctxToken, cookie.Value)
			}
		})
	}
}

func TestCSRF_ReusesExistingToken(t *testing.T) {
	t.Parallel()

	c := middleware.NewCSRF(&config.Config{Env: "test"})

	var called bool
	var ctxToken string

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: "existing-token"})
	rec := httptest.NewRecorder()

	c.Middleware(newCSRFRecorder(&called, &ctxToken)).ServeHTTP(rec, req)

	if ctxToken != "existing-token" {
		t.Errorf("context のトークン = %q, want %q", ctxToken, "existing-token")
	}
	// No new cookie is issued when a valid one already exists.
	//
	// [Ja] 有効な Cookie が既にあるとき新しい Cookie は発行しない。
	if cookie := findCookie(rec, middleware.CSRFCookieName); cookie != nil {
		t.Errorf("既存トークンがあるのに新しい Cookie %q が発行された", middleware.CSRFCookieName)
	}
}

func TestCSRF_CookieAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		wantSecure bool
	}{
		{name: "本番では Secure を立てる", env: "prod", wantSecure: true},
		{name: "テストでは Secure を立てない (平文 HTTP のため)", env: "test", wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := middleware.NewCSRF(&config.Config{Env: tt.env})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			var called bool
			var ctxToken string
			c.Middleware(newCSRFRecorder(&called, &ctxToken)).ServeHTTP(rec, req)

			cookie := findCookie(rec, middleware.CSRFCookieName)
			if cookie == nil {
				t.Fatalf("CSRF Cookie %q が発行されていない", middleware.CSRFCookieName)
			}
			if cookie.Secure != tt.wantSecure {
				t.Errorf("cookie.Secure = %v, want %v", cookie.Secure, tt.wantSecure)
			}
			if cookie.HttpOnly {
				t.Error("CSRF Cookie は JavaScript から読めるよう HttpOnly ではないべき")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
			}
		})
	}
}

func TestCSRF_Verify(t *testing.T) {
	t.Parallel()

	const token = "valid-csrf-token"

	c := middleware.NewCSRF(&config.Config{Env: "test"})

	tests := []struct {
		name   string
		method string
		// empty means no cookie is sent.
		//
		// [Ja] 空のときは Cookie を送らない
		cookieToken string
		formToken   string
		headerToken string
		wantStatus  int
	}{
		{name: "POST: フォームトークンが一致すれば通す", method: http.MethodPost, cookieToken: token, formToken: token, wantStatus: http.StatusOK},
		{name: "POST: ヘッダートークンが一致すれば通す", method: http.MethodPost, cookieToken: token, headerToken: token, wantStatus: http.StatusOK},
		// PATCH bodies are read by ParseForm, so the form token works. A direct
		// DELETE body is not (a form-driven DELETE arrives via method-override,
		// whose ParseForm already cached the body while the method was POST), so
		// a standalone DELETE carries its token in the header like an AJAX call.
		//
		// [Ja] PATCH のボディは ParseForm が読むためフォームトークンが効く。直接の
		// DELETE のボディは読まれない (フォーム由来の DELETE は method-override 経由で
		// 来て、その ParseForm が POST のうちにボディをキャッシュ済み) ため、単独の
		// DELETE は AJAX 同様ヘッダーでトークンを運ぶ。
		{name: "PATCH: フォームトークンが一致すれば通す", method: http.MethodPatch, cookieToken: token, formToken: token, wantStatus: http.StatusOK},
		{name: "DELETE: ヘッダートークンが一致すれば通す", method: http.MethodDelete, cookieToken: token, headerToken: token, wantStatus: http.StatusOK},
		{name: "POST: Cookie が無ければ 403", method: http.MethodPost, cookieToken: "", formToken: token, wantStatus: http.StatusForbidden},
		{name: "POST: フォームにもヘッダーにもトークンが無ければ 403", method: http.MethodPost, cookieToken: token, wantStatus: http.StatusForbidden},
		{name: "POST: トークンが一致しなければ 403", method: http.MethodPost, cookieToken: token, formToken: "wrong-token", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var body string
			if tt.formToken != "" {
				body = "csrf_token=" + tt.formToken
			}
			req := httptest.NewRequest(tt.method, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tt.cookieToken != "" {
				req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: tt.cookieToken})
			}
			if tt.headerToken != "" {
				req.Header.Set("X-CSRF-Token", tt.headerToken)
			}
			rec := httptest.NewRecorder()

			var called bool
			var ctxToken string
			c.Middleware(newCSRFRecorder(&called, &ctxToken)).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if !called {
					t.Fatal("検証成功時は次のハンドラーが呼ばれるべき")
				}
				// The verified token is stored so a re-rendered form can reuse it.
				//
				// [Ja] 再描画フォームが再利用できるよう検証済みトークンを格納する。
				if ctxToken != tt.cookieToken {
					t.Errorf("検証済みトークンが context に格納されていない: got %q, want %q", ctxToken, tt.cookieToken)
				}
			} else if called {
				t.Error("検証失敗時は次のハンドラーを呼ばないべき")
			}
		})
	}
}

func TestCSRF_FormDrivenDeleteViaMethodOverride(t *testing.T) {
	t.Parallel()

	const token = "valid-csrf-token"

	c := middleware.NewCSRF(&config.Config{Env: "test"})

	var called bool
	var gotMethod string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	})

	// A form-driven DELETE arrives as a POST carrying _method=DELETE and the CSRF
	// token in the body. MethodOverride parses the body (while the method is still
	// POST) and flips the method to DELETE; CSRF then reads the token from the
	// already-parsed form, so the chain accepts the request.
	//
	// [Ja] フォーム由来の DELETE は _method=DELETE と CSRF トークンをボディに載せた
	// POST として届く。MethodOverride が (メソッドが POST のうちに) ボディを解析して
	// メソッドを DELETE へ反転させ、CSRF は解析済みフォームからトークンを読むため、
	// チェーン全体がリクエストを受理する。
	body := "_method=DELETE&csrf_token=" + token
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
	rec := httptest.NewRecorder()

	middleware.MethodOverride(c.Middleware(next)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("r.Method = %q, want %q", gotMethod, http.MethodDelete)
	}
}

func TestCSRFTokenFromContext_NotSet(t *testing.T) {
	t.Parallel()

	if token := middleware.CSRFTokenFromContext(context.Background()); token != "" {
		t.Errorf("CSRFTokenFromContext() = %q, want empty", token)
	}
}
