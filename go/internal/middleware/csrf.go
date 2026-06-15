package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
)

// CSRFCookieName is the name of the cookie that stores the CSRF token. It carries
// the project prefix per the identifier naming convention.
//
// [Ja] CSRFCookieName は CSRF トークンを格納する Cookie の名前です。識別子の命名
// 規約に従いプロジェクト接頭辞を付けています。
const CSRFCookieName = "groobb_csrf_token"

// csrfCookieMaxAge is the CSRF cookie lifetime in seconds (24 hours). When the
// cookie expires a fresh token is minted on the next safe request, so the form
// the user is about to submit always has a matching cookie to validate against.
//
// [Ja] csrfCookieMaxAge は CSRF Cookie の有効期間 (秒、24 時間) です。Cookie が失効
// しても次の安全なリクエストで新しいトークンが発行されるため、ユーザーがこれから
// 送信するフォームには照合に使える Cookie が常に存在します。
const csrfCookieMaxAge = 24 * 60 * 60

// csrfTokenContextKey is the key under which the CSRF token is stored in the
// request context. It reuses the contextKey type defined in auth.go.
//
// [Ja] csrfTokenContextKey は CSRF トークンをリクエスト context に格納する際の
// キーです。auth.go で定義した contextKey 型を再利用します。
const csrfTokenContextKey contextKey = "csrf_token"

// CSRF guards state-changing requests with a double-submit-cookie CSRF check: a
// random token is stored in a cookie and must be echoed back in the form (or the
// X-CSRF-Token header) of every unsafe request. A double-submit cookie is used
// rather than a server-side session token because Groobb's sessions are bound to
// a signed-in user, while the forms that most need protection (sign-in, sign-up)
// are submitted by anonymous visitors who have no session yet.
//
// [Ja] CSRF は double-submit cookie 方式の CSRF 検証で状態変更リクエストを保護します。
// ランダムなトークンを Cookie に保存し、すべての安全でないリクエストのフォーム
// (または X-CSRF-Token ヘッダー) で同じトークンを返させます。サーバー側のセッション
// トークンではなく double-submit cookie を使うのは、Groobb のセッションがサインイン
// 済みユーザーに紐づくのに対し、最も保護が必要なフォーム (サインイン・サインアップ)
// はまだセッションを持たない匿名訪問者が送信するためです。
type CSRF struct {
	cfg *config.Config
}

// NewCSRF creates a CSRF middleware.
// [Ja] NewCSRF は CSRF ミドルウェアを生成します。
func NewCSRF(cfg *config.Config) *CSRF {
	return &CSRF{cfg: cfg}
}

// Middleware issues a CSRF token on safe requests and verifies it on unsafe ones.
// Safe methods (GET / HEAD / OPTIONS) carry no side effects, so they mint or
// reuse the token, store it in the context for templates to embed in forms, and
// pass through. Every other method must present a token that matches the cookie,
// otherwise the request is rejected with 403 before reaching the handler.
//
// [Ja] Middleware は安全なリクエストで CSRF トークンを発行し、安全でないリクエストで
// 検証します。安全なメソッド (GET / HEAD / OPTIONS) は副作用を持たないため、トークンを
// 発行または再利用し、テンプレートがフォームに埋め込めるよう context へ格納して
// 素通しします。それ以外のメソッドは Cookie と一致するトークンの提示が必須で、
// 一致しなければハンドラーに到達する前に 403 で拒否します。
func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			token, err := c.getOrCreateToken(w, r)
			if err != nil {
				// Token generation failed (rand.Read). Log before returning 500
				// so this rare failure still leaves a trace for debugging.
				//
				// [Ja] トークン生成 (rand.Read) に失敗。まれな失敗でも調査の手がかりを
				// 残すため、500 を返す前に記録する。
				slog.ErrorContext(r.Context(), "CSRF トークンの生成に失敗", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			ctx := context.WithValue(r.Context(), csrfTokenContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		cookie, err := r.Cookie(CSRFCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Read the submitted token from the form first, then fall back to the
		// header so AJAX requests can authenticate without a form field.
		//
		// [Ja] 送信されたトークンをまずフォームから読み、無ければヘッダーへフォール
		// バックする。AJAX リクエストがフォームフィールド無しでも認証できるようにする。
		requestToken := r.FormValue("csrf_token")
		if requestToken == "" {
			requestToken = r.Header.Get("X-CSRF-Token")
		}

		if requestToken == "" || requestToken != cookie.Value {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Store the verified token so a handler re-rendering its form (e.g. after a
		// validation error) can place the same token back into the form.
		//
		// [Ja] フォームを再描画するハンドラー (例: バリデーションエラー後) が同じ
		// トークンをフォームに戻せるよう、検証済みトークンを context へ格納する。
		ctx := context.WithValue(r.Context(), csrfTokenContextKey, cookie.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getOrCreateToken returns the token carried by the request cookie, minting and
// setting a new one when the cookie is absent or empty.
//
// [Ja] getOrCreateToken はリクエスト Cookie が持つトークンを返し、Cookie が無い /
// 空のときは新しいトークンを発行して設定します。
func (c *CSRF) getOrCreateToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(CSRFCookieName); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	token, err := generateCSRFToken()
	if err != nil {
		return "", err
	}

	c.setCookie(w, token)
	return token, nil
}

// setCookie writes the CSRF token to the cookie. Unlike the session cookie it is
// not HttpOnly so client-side scripts can echo it back in the X-CSRF-Token
// header; Secure is on only in production so it still works over plain HTTP in
// dev / test, matching the session cookie's policy.
//
// [Ja] setCookie は CSRF トークンを Cookie に書き込みます。セッション Cookie と異なり
// HttpOnly ではないため、クライアント側スクリプトが X-CSRF-Token ヘッダーで同じ
// トークンを返せます。Secure は本番でのみ有効にし、dev / test では平文 HTTP でも
// 機能するようにします (セッション Cookie の方針に揃えています)。
func (c *CSRF) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		Secure:   c.cfg.IsProduction(),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   csrfCookieMaxAge,
	})
}

// CSRFTokenFromContext returns the CSRF token stored by Middleware, or "" when
// none is present (e.g. Middleware did not run). Templates pass it into the
// hidden csrf_token field of every form.
//
// [Ja] CSRFTokenFromContext は Middleware が格納した CSRF トークンを返します。無い
// とき (Middleware が走っていないときなど) は "" を返します。テンプレートはこれを
// 各フォームの hidden な csrf_token フィールドに渡します。
func CSRFTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(csrfTokenContextKey).(string)
	return token
}

// generateCSRFToken returns a 32-byte cryptographically random token,
// base64-encoded so it is safe to use as a cookie value.
//
// [Ja] generateCSRFToken は 32 バイトの暗号論的乱数トークンを返します。Cookie 値と
// して安全に使えるよう base64 エンコードします。
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
