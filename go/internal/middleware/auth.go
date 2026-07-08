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
//
// [Ja] Auth は認証ミドルウェアの依存を保持する。
type Auth struct {
	sessionMgr *session.Manager
}

// NewAuth creates an Auth middleware.
//
// [Ja] NewAuth は Auth ミドルウェアを生成する。
func NewAuth(sessionMgr *session.Manager) *Auth {
	return &Auth{sessionMgr: sessionMgr}
}

// RequireAuth guards routes that require an authenticated user. It resolves the
// current user from the session cookie and, when signed in, stores it in the
// request context before handing off to next. An anonymous request (no cookie,
// or an unknown or stale token) is redirected to /sign_in instead of reaching
// the handler. Unlike SetUser, a genuine lookup failure (e.g. the database is
// unreachable) is treated as fatal and answered with 500, because a protected
// page cannot be rendered safely without knowing who the visitor is.
//
// [Ja] RequireAuth は認証済みユーザーを要求するルートを保護する。セッション Cookie
// から現在のユーザーを解決し、サインイン済みのときは next へ渡す前にリクエスト context
// に格納する。匿名リクエスト (Cookie が無い / token が未知・失効) はハンドラーに到達
// させず /sign_in へリダイレクトする。SetUser と異なり、本物の解決失敗 (例: データ
// ベースに到達できない) は致命的として 500 で応答する。保護されたページは訪問者が誰か
// わからないまま安全に描画できないためである。
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		user, err := a.sessionMgr.GetCurrentUser(ctx, r)
		if err != nil {
			slog.ErrorContext(ctx, "認証チェック中にエラーが発生", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if user == nil {
			http.Redirect(w, r, "/sign_in", http.StatusSeeOther)
			return
		}

		ctx = context.WithValue(ctx, userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

// SetUserToContext returns a copy of ctx carrying user as the current user. It
// stores the user under the same unexported key SetUser and RequireAuth use, so
// UserFromContext reads it back. This lets a caller (chiefly a handler test)
// exercise a handler that depends on UserFromContext without routing the request
// through the auth middleware.
//
// [Ja] SetUserToContext は user を現在のユーザーとして載せた ctx のコピーを返す。
// SetUser / RequireAuth と同じ非公開キーでユーザーを格納するため、UserFromContext
// が読み戻せる。これにより呼び出し元 (主にハンドラーテスト) は、リクエストを認証
// ミドルウェアに通さずに UserFromContext に依存するハンドラーを試せる。
func SetUserToContext(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
