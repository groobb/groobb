// Package session manages Cookie-backed database sessions and flash messages:
// it resolves the current user from a request's session cookie, sets and clears
// that cookie, and carries one-shot flash messages across redirects.
//
// [Ja] session パッケージは Cookie ベースの DB セッションとフラッシュメッセージを
// 管理します。リクエストのセッション Cookie から現在のユーザーを解決し、その Cookie
// を設定・削除し、リダイレクトをまたいで一度きりのフラッシュメッセージを運びます。
package session

import (
	"context"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// CookieName is the name of the cookie that stores the session token. It carries
// the project prefix per the environment/identifier naming convention.
//
// [Ja] CookieName はセッショントークンを格納する Cookie の名前です。環境変数 /
// 識別子の命名規約に従いプロジェクト接頭辞を付けています。
const CookieName = "groobb_session_token"

// sessionCookieMaxAge is the session cookie lifetime in seconds. Sessions have
// no server-side expiry column (a row in user_sessions lives until explicit
// sign-out), so the cookie is intentionally long-lived ("stay signed in until
// you sign out"). The value matches the sister Korylus projects for
// cross-project consistency.
//
// [Ja] sessionCookieMaxAge はセッション Cookie の有効期間 (秒) です。セッションには
// サーバー側の有効期限カラムが無く (user_sessions の行は明示的なサインアウトまで
// 生存する) ため、Cookie は意図的に長寿命にしています (「サインアウトするまで
// ログイン状態を保つ」)。値はプロジェクト間の一貫性のため姉妹 Korylus プロジェクトに
// 揃えています。
const sessionCookieMaxAge = 10 * 365 * 24 * 60 * 60

// Manager resolves the current user from the session cookie and manages that
// cookie's lifecycle. It does not create sessions itself: persisting a session
// row is the sign-in UseCase's job, which then calls SetSessionCookie.
//
// [Ja] Manager はセッション Cookie から現在のユーザーを解決し、その Cookie の
// ライフサイクルを管理します。セッションの作成自体は行いません。セッション行の
// 永続化はサインイン UseCase の責務で、その後に SetSessionCookie を呼びます。
type Manager struct {
	userRepo *repository.UserRepository
	cfg      *config.Config
}

// NewManager creates a Manager.
//
// [Ja] NewManager は Manager を生成します。
func NewManager(
	userRepo *repository.UserRepository,
	cfg *config.Config,
) *Manager {
	return &Manager{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// GetCurrentUser resolves the request's session cookie to the signed-in user,
// returning (nil, nil) when the request is not signed in: no cookie, or an
// unknown or stale token. A non-nil error is reserved for genuine failures
// (e.g. the database is unreachable).
//
// [Ja] GetCurrentUser はリクエストのセッション Cookie をサインイン済みユーザーに
// 解決します。未サインインのとき (Cookie が無い / token が未知・失効) は (nil, nil)
// を返します。非 nil のエラーは本物の失敗 (例: データベースに到達できない) のために
// のみ用います。
func (m *Manager) GetCurrentUser(ctx context.Context, r *http.Request) (*model.User, error) {
	token := m.sessionToken(r)
	if token == "" {
		return nil, nil
	}

	return m.userRepo.FindBySessionToken(ctx, token)
}

// SetSessionCookie writes the session token to the session cookie. Secure is on
// only in production so the cookie still works over plain HTTP in dev / test;
// HttpOnly keeps it out of reach of JavaScript, and SameSite=Lax limits it on
// cross-site requests.
//
// [Ja] SetSessionCookie はセッショントークンをセッション Cookie に書き込みます。
// Secure は本番でのみ有効にし、dev / test では平文 HTTP でも Cookie が機能する
// ようにします。HttpOnly で JavaScript から触れないようにし、SameSite=Lax で
// クロスサイトリクエストでの送出を制限します。
func (m *Manager) SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionCookieMaxAge,
	})
}

// DeleteSessionCookie clears the session cookie by setting a matching cookie
// with MaxAge < 0, which instructs the browser to delete it. The other
// attributes mirror SetSessionCookie so the browser matches and removes it.
//
// [Ja] DeleteSessionCookie は MaxAge < 0 の同名 Cookie を設定してセッション Cookie を
// 消去します (ブラウザに削除を指示する)。他の属性は SetSessionCookie と揃え、
// ブラウザが一致して削除できるようにします。
func (m *Manager) DeleteSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// sessionToken returns the session token from the request cookie, or "" when the
// cookie is absent.
//
// [Ja] sessionToken はリクエスト Cookie からセッショントークンを返します。Cookie が
// 無い場合は "" を返します。
func (m *Manager) sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
