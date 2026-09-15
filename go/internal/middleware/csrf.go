package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
)

// CSRFCookieNameはCSRFトークンを格納するCookieの名前です。識別子の命名
// 規約に従いプロジェクト接頭辞を付けています。
const CSRFCookieName = "groobb_csrf_token"

// csrfCookieMaxAgeはCSRF Cookieの有効期間 (秒、24時間) です。Cookieが失効
// しても次の安全なリクエストで新しいトークンが発行されるため、ユーザーがこれから
// 送信するフォームには照合に使えるCookieが常に存在します。
const csrfCookieMaxAge = 24 * 60 * 60

// csrfTokenContextKeyはCSRFトークンをリクエストcontextに格納する際の
// キーです。auth.goで定義したcontextKey型を再利用します。
const csrfTokenContextKey contextKey = "csrf_token"

// CSRFはdouble-submit cookie方式のCSRF検証で状態変更リクエストを保護します。
// ランダムなトークンをCookieに保存し、すべての安全でないリクエストのフォーム
// (またはX-CSRF-Tokenヘッダー) で同じトークンを返させます。サーバー側のセッション
// トークンではなくdouble-submit cookieを使うのは、Groobbのセッションがサインイン
// 済みユーザーに紐づくのに対し、最も保護が必要なフォーム (サインイン・サインアップ)
// はまだセッションを持たない匿名訪問者が送信するためです。
type CSRF struct {
	cfg *config.Config
}

// NewCSRFはCSRFミドルウェアを生成します。
func NewCSRF(cfg *config.Config) *CSRF {
	return &CSRF{cfg: cfg}
}

// Middlewareは安全なリクエストでCSRFトークンを発行し、安全でないリクエストで
// 検証します。安全なメソッド (GET / HEAD / OPTIONS) は副作用を持たないため、トークンを
// 発行または再利用し、テンプレートがフォームに埋め込めるようcontextへ格納して
// 素通しします。それ以外のメソッドはCookieと一致するトークンの提示が必須で、
// 一致しなければハンドラーに到達する前に403で拒否します。
func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			token, err := c.getOrCreateToken(w, r)
			if err != nil {
				// トークン生成 (rand.Read) に失敗。まれな失敗でも調査の手がかりを
				// 残すため、500を返す前に記録する。
				slog.ErrorContext(r.Context(), "CSRFトークンの生成に失敗", "error", err)
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

		// 送信されたトークンをまずフォームから読み、無ければヘッダーへフォール
		// バックする。AJAXリクエストがフォームフィールド無しでも認証できるようにする。
		requestToken := r.FormValue("csrf_token")
		if requestToken == "" {
			requestToken = r.Header.Get("X-CSRF-Token")
		}

		if requestToken == "" || requestToken != cookie.Value {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// フォームを再描画するハンドラー (例: バリデーションエラー後) が同じ
		// トークンをフォームに戻せるよう、検証済みトークンをcontextへ格納する。
		ctx := context.WithValue(r.Context(), csrfTokenContextKey, cookie.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getOrCreateTokenはリクエストCookieが持つトークンを返し、Cookieが無い /
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

// setCookieはCSRFトークンをCookieに書き込みます。セッションCookieと異なり
// HttpOnlyではないため、クライアント側スクリプトがX-CSRF-Tokenヘッダーで同じ
// トークンを返せます。Secureは本番でのみ有効にし、dev / testでは平文HTTPでも
// 機能するようにします (セッションCookieの方針に揃えています)。
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

// CSRFTokenFromContextはMiddlewareが格納したCSRFトークンを返します。無い
// とき (Middlewareが走っていないときなど) は "" を返します。テンプレートはこれを
// 各フォームのhiddenなcsrf_tokenフィールドに渡します。
func CSRFTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(csrfTokenContextKey).(string)
	return token
}

// generateCSRFTokenは32バイトの暗号論的乱数トークンを返します。Cookie値と
// して安全に使えるようbase64エンコードします。
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
