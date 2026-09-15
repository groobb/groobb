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

// newCSRFRecorderは実行されたかどうかとcontextから見えるCSRFトークンを
// 記録するハンドラーを返す。各ケースでMiddlewareが何を素通し・格納したかを
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
				t.Errorf("contextのトークン = %q、期待値 = %q (発行したCookieと一致)", ctxToken, cookie.Value)
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
		t.Errorf("contextのトークン = %q、期待値 = %q", ctxToken, "existing-token")
	}
	// 有効なCookieが既にあるとき新しいCookieは発行しない。
	if cookie := findCookie(rec, middleware.CSRFCookieName); cookie != nil {
		t.Errorf("既存トークンがあるのに新しいCookie %q が発行された", middleware.CSRFCookieName)
	}
}

func TestCSRF_CookieAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		wantSecure bool
	}{
		{name: "本番ではSecureを立てる", env: "prod", wantSecure: true},
		{name: "テストではSecureを立てない (平文HTTPのため)", env: "test", wantSecure: false},
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
				t.Errorf("cookie.Secure = %v、期待値 = %v", cookie.Secure, tt.wantSecure)
			}
			if cookie.HttpOnly {
				t.Error("CSRF CookieがHttpOnlyになっている (JavaScriptから読めるようHttpOnlyでないことを期待)")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v、期待値 = %v", cookie.SameSite, http.SameSiteLaxMode)
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
		// 空のときはCookieを送らない
		cookieToken string
		formToken   string
		headerToken string
		wantStatus  int
	}{
		{name: "POST: フォームトークンが一致すれば通す", method: http.MethodPost, cookieToken: token, formToken: token, wantStatus: http.StatusOK},
		{name: "POST: ヘッダートークンが一致すれば通す", method: http.MethodPost, cookieToken: token, headerToken: token, wantStatus: http.StatusOK},
		// PATCHのボディはParseFormが読むためフォームトークンが効く。直接の
		// DELETEのボディは読まれない (フォーム由来のDELETEはmethod-override経由で
		// 来て、そのParseFormがPOSTのうちにボディをキャッシュ済み) ため、単独の
		// DELETEはAJAX同様ヘッダーでトークンを運ぶ。
		{name: "PATCH: フォームトークンが一致すれば通す", method: http.MethodPatch, cookieToken: token, formToken: token, wantStatus: http.StatusOK},
		{name: "DELETE: ヘッダートークンが一致すれば通す", method: http.MethodDelete, cookieToken: token, headerToken: token, wantStatus: http.StatusOK},
		{name: "POST: Cookieが無ければ403", method: http.MethodPost, cookieToken: "", formToken: token, wantStatus: http.StatusForbidden},
		{name: "POST: フォームにもヘッダーにもトークンが無ければ403", method: http.MethodPost, cookieToken: token, wantStatus: http.StatusForbidden},
		{name: "POST: トークンが一致しなければ403", method: http.MethodPost, cookieToken: token, formToken: "wrong-token", wantStatus: http.StatusForbidden},
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
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if !called {
					t.Fatal("検証成功時に次のハンドラーが呼ばれなかった (呼ばれることを期待)")
				}
				// 再描画フォームが再利用できるよう検証済みトークンを格納する。
				if ctxToken != tt.cookieToken {
					t.Errorf("検証済みトークンがcontextに格納されていない: 実測値 = %q、期待値 = %q", ctxToken, tt.cookieToken)
				}
			} else if called {
				t.Error("検証失敗時に次のハンドラーが呼ばれた (呼ばれないことを期待)")
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

	// フォーム由来のDELETEは _method=DELETEとCSRFトークンをボディに載せた
	// POSTとして届く。MethodOverrideが (メソッドがPOSTのうちに) ボディを解析して
	// メソッドをDELETEへ反転させ、CSRFは解析済みフォームからトークンを読むため、
	// チェーン全体がリクエストを受理する。
	body := "_method=DELETE&csrf_token=" + token
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
	rec := httptest.NewRecorder()

	middleware.MethodOverride(c.Middleware(next)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("r.Method = %q、期待値 = %q", gotMethod, http.MethodDelete)
	}
}

func TestCSRFTokenFromContext_NotSet(t *testing.T) {
	t.Parallel()

	if token := middleware.CSRFTokenFromContext(context.Background()); token != "" {
		t.Errorf("CSRFTokenFromContext() = %q、期待値は空文字列", token)
	}
}
