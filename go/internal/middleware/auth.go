// middlewareパッケージはHTTPミドルウェアを提供します。本ファイルはセッション
// Cookieから現在のユーザーを解決する認証ミドルウェアを担います。
package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
)

// contextKeyはcontextキー用の非公開型で、他パッケージで定義されたキーとの
// 衝突を避けるために用いる。
type contextKey string

// userContextKeyは現在のユーザーをリクエストcontextに格納する際のキー。
const userContextKey contextKey = "user"

// Authは認証ミドルウェアの依存を保持する。
type Auth struct {
	sessionMgr *session.Manager
}

// NewAuthはAuthミドルウェアを生成する。
func NewAuth(sessionMgr *session.Manager) *Auth {
	return &Auth{sessionMgr: sessionMgr}
}

// RequireAuthは認証済みユーザーを要求するルートを保護する。セッションCookie
// から現在のユーザーを解決し、サインイン済みのときはnextへ渡す前にリクエストcontext
// に格納する。匿名のGET / HEADリクエスト (Cookieが無い / tokenが未知・失効) は
// ハンドラーに到達させず /sign_inへリダイレクトし、サインイン後に元のURLへ戻れるよう
// リクエスト先をreturn_toとして載せる。そうしないと、保護されたページの共有リンクを
// 未サインインで踏んだ人はそのページに辿り着けない。その他のメソッドは、宛先をサインイン後に
// GETで安全に再現できないため、素の /sign_inへフォールバックする。SetUserと異なり、
// 本物の解決失敗 (例: データベースに到達できない) は致命的として500で応答する。保護された
// ページは訪問者が誰かわからないまま安全に描画できないためである。本ミドルウェアが返す
// すべてのレスポンスにはCache-Control: private, no-cacheを付ける。
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 保護されたルートの応答は誰が要求したかで変わる (サインイン済みならページ
		// 自身、それ以外はサインインへのリダイレクト) ため、ここを出るどのレスポンスも
		// 共有キャッシュに保持されたり再検証なしで再利用されたりしてはならない。各
		// ハンドラーではなくここで設定することで、後から追加したルートも保護されている
		// こと自体で方針を備え、値はページだけでなくリダイレクトと500にも届く。より
		// 厳しい方針が要るハンドラーは値を置き換える (settings_two_factor_authは平文の
		// secretとリカバリーコードのためにno-storeを使う)。
		w.Header().Set("Cache-Control", "private, no-cache")

		user, err := a.sessionMgr.GetCurrentUser(ctx, r)
		if err != nil {
			slog.ErrorContext(ctx, "認証チェック中にエラーが発生", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if user == nil {
			http.Redirect(w, r, signInPathWithReturnTo(r), http.StatusSeeOther)
			return
		}

		ctx = context.WithValue(ctx, userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetUserはリクエストのセッションCookieから現在のユーザーを解決し、サイン
// イン済みのときは後続のハンドラーやテンプレートが参照できるようリクエストcontext
// に格納する。リクエストを止めることはしない。匿名リクエスト (Cookieが無い / token
// が未知・失効) はcontextにユーザーを入れずに進み、本物の解決失敗 (例: データベース
// に到達できない) はログに記録したうえで匿名のまま進める。これにより一時的なデータ
// ベースの不調が、匿名の訪問者でも見られるページを巻き込んで落とさないようにする。
// 認証の強制は別の関心事であり、ルート単位のRequireAuthが担う。
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

// UserFromContextはctxに格納された現在のユーザーを返す。未サインインの
// とき (またはSetUserが走っていないとき) はnilを返す。
func UserFromContext(ctx context.Context) *model.User {
	user, ok := ctx.Value(userContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}

// UserIDFromContextは現在のユーザーのidを返し、リクエストがサインイン済みの
// ユーザーを運んでいないときはnilを返します。誰が見ているかだけを必要とするものへ
// 渡す値であり、nilの判定を、サインイン状態でもサインアウト状態でも到達するルート
// ごとに書かず、ここに1度だけ置くためのものです。
func UserIDFromContext(ctx context.Context) *model.UserID {
	user := UserFromContext(ctx)
	if user == nil {
		return nil
	}
	// idはアドレスを取る前に写します。呼び出し側が持つものが、リクエストの運ぶ
	// ユーザーへの入口にならないようにするためです。
	userID := user.ID
	return &userID
}

// SetUserToContextはuserを現在のユーザーとして載せたctxのコピーを返す。
// SetUser / RequireAuthと同じ非公開キーでユーザーを格納するため、UserFromContext
// が読み戻せる。これにより呼び出し元 (主にハンドラーテスト) は、リクエストを認証
// ミドルウェアに通さずにUserFromContextに依存するハンドラーを試せる。
func SetUserToContext(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
