// Package middleware provides HTTP middleware. This file holds the
// authentication middleware that resolves the current user from the session
// cookie.
//
// [Ja] middleware パッケージは HTTP ミドルウェアを提供します。本ファイルはセッション
// Cookie から現在のユーザーを解決する認証ミドルウェアを担います。
package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
)

// contextKey is an unexported type for context keys to avoid collisions with
// keys defined in other packages.
//
// [Ja] contextKey は context キー用の非公開型で、他パッケージで定義されたキーとの
// 衝突を避けるために用いる。
type contextKey string

// userContextKey is the key under which the current user is stored in the
// request context.
//
// [Ja] userContextKey は現在のユーザーをリクエスト context に格納する際のキー。
const userContextKey contextKey = "user"

// Auth holds the dependencies of the authentication middleware.
// [Ja] Auth は認証ミドルウェアの依存を保持する。
type Auth struct {
	sessionMgr *session.Manager
}

// NewAuth creates an Auth middleware.
// [Ja] NewAuth は Auth ミドルウェアを生成する。
func NewAuth(sessionMgr *session.Manager) *Auth {
	return &Auth{sessionMgr: sessionMgr}
}

// SetUser resolves the current user from the request's session cookie and, when
// signed in, stores it in the request context for downstream handlers and
// templates. It never blocks the request: an anonymous request (no cookie, or an
// unknown or stale token) proceeds with no user in the context, and a genuine
// lookup failure (e.g. the database is unreachable) is logged and the request
// still proceeds anonymously, so a transient database hiccup does not take down
// pages that anonymous visitors can see. Enforcing authentication is a separate
// concern handled by route-level middleware added in later tasks.
//
// [Ja] SetUser はリクエストのセッション Cookie から現在のユーザーを解決し、サイン
// イン済みのときは後続のハンドラーやテンプレートが参照できるようリクエスト context
// に格納する。リクエストを止めることはしない。匿名リクエスト (Cookie が無い / token
// が未知・失効) は context にユーザーを入れずに進み、本物の解決失敗 (例: データベース
// に到達できない) はログに記録したうえで匿名のまま進める。これにより一時的なデータ
// ベースの不調が、匿名の訪問者でも見られるページを巻き込んで落とさないようにする。
// 認証の強制は別の関心事であり、後続タスクで追加するルート単位のミドルウェアが担う。
func (a *Auth) SetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		user, err := a.sessionMgr.GetCurrentUser(ctx, r)
		if err != nil {
			slog.WarnContext(ctx, "現在のユーザーの解決に失敗", "error", err)
			next.ServeHTTP(w, r)
			return
		}

		if user != nil {
			ctx = context.WithValue(ctx, userContextKey, user)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromContext returns the current user stored in ctx, or nil when the
// request is not signed in (or SetUser did not run).
//
// [Ja] UserFromContext は ctx に格納された現在のユーザーを返す。未サインインの
// とき (または SetUser が走っていないとき) は nil を返す。
func UserFromContext(ctx context.Context) *model.User {
	user, ok := ctx.Value(userContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}
