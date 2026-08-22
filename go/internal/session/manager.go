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
	"time"

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

// EmailConfirmationCookieName is the name of the cookie that carries a signed
// continuation token for the pending email confirmation across the sign-up
// handoff. The token binds the confirmation id, purpose, and expiry so the
// code-entry step can identify the confirmation without trusting a browser-
// supplied id. It carries the project prefix per the identifier naming
// convention.
//
// [Ja] EmailConfirmationCookieName は、サインアップの受け渡しをまたいで保留中のメール
// 確認用の署名付き continuation token を運ぶ Cookie の名前です。token は確認 id・用途・
// 有効期限を結び付け、コード入力のステップがブラウザー提供の id を信頼せず確認を特定
// できるようにします。識別子の命名規約に従いプロジェクト接頭辞を付けています。
const EmailConfirmationCookieName = "groobb_email_confirmation"

// TwoFactorPendingCookieName is the name of the cookie that carries a signed
// continuation token across the two-factor sign-in handoff. After the password
// check passes, the token binds the pending user's id, purpose, and expiry; the
// TOTP / recovery-code challenge verifies it before choosing whose second factor
// to check. It carries the project prefix per the identifier naming convention.
//
// [Ja] TwoFactorPendingCookieName は、2 段階認証のサインイン受け渡しをまたいで署名付き
// continuation token を運ぶ Cookie の名前です。パスワード検証後に token が保留中ユーザーの
// id・用途・有効期限を結び付け、TOTP / リカバリーコードのチャレンジは誰の第 2 要素を調べるか
// 選ぶ前にそれを検証します。識別子の命名規約に従いプロジェクト接頭辞を付けています。
const TwoFactorPendingCookieName = "groobb_two_factor_pending"

// emailConfirmationCookieMaxAge is the email-confirmation cookie lifetime in
// seconds (15 minutes). It matches the confirmation code's own expiry window
// (the Korylus convention is 15 minutes), so the cookie does not outlive the
// code it points at.
//
// [Ja] emailConfirmationCookieMaxAge はメール確認 Cookie の有効期間 (秒、15 分) です。
// 確認コード自体の有効期限ウィンドウ (Korylus の慣行は 15 分) に揃え、Cookie が指す先の
// コードより長く生存しないようにします。
const emailConfirmationCookieMaxAge = 15 * 60

// twoFactorPendingCookieMaxAge is the two-factor pending cookie lifetime in
// seconds (10 minutes). The window is deliberately short: it only needs to span
// entering a TOTP or recovery code right after the password step, and a short
// life limits how long a leaked pending cookie stays useful.
//
// [Ja] twoFactorPendingCookieMaxAge は 2 段階認証の pending Cookie の有効期間 (秒、10 分)
// です。ウィンドウは意図的に短くしています。パスワードのステップ直後に TOTP または
// リカバリーコードを入力する間だけを賄えればよく、短い寿命は漏えいした pending Cookie が
// 有用でいられる時間を抑えます。
const twoFactorPendingCookieMaxAge = 10 * 60

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
	token := m.SessionToken(r)
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

// SetEmailConfirmationID writes a signed, purpose-bound continuation token for
// the pending confirmation to its cookie. Its signed expiry matches MaxAge so
// expiry is enforced by the server as well as the browser. The cookie is
// HttpOnly because it carries server-side state; the other attributes mirror the
// session cookie's policy (Secure only in production, SameSite=Lax).
//
// [Ja] SetEmailConfirmationID は保留中の確認用に、署名され用途を結び付けた continuation
// token を専用 Cookie へ書き込みます。署名対象の有効期限を MaxAge と揃え、ブラウザーだけで
// なくサーバー側でも期限を強制します。サーバー側状態を運ぶため Cookie は HttpOnly とし、
// 他の属性はセッション Cookie の方針 (Secure は本番のみ・SameSite=Lax) に揃えます。
func (m *Manager) SetEmailConfirmationID(w http.ResponseWriter, id model.EmailConfirmationID) {
	expiresAt := time.Now().Add(emailConfirmationCookieMaxAge * time.Second)
	token := signContinuationToken(m.cfg.ContinuationTokenKey, emailConfirmationTokenPurpose, int64(id), expiresAt)

	http.SetCookie(w, &http.Cookie{
		Name:     EmailConfirmationCookieName,
		Value:    token,
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   emailConfirmationCookieMaxAge,
	})
}

// GetEmailConfirmationID returns the pending confirmation id authenticated by
// the request cookie's continuation token. The second return value is false when
// the cookie is absent, malformed, forged, for another purpose, or expired.
//
// [Ja] GetEmailConfirmationID はリクエスト Cookie の continuation token で認証された
// 保留中の確認 id を返します。Cookie が無い、形式不正、偽造、別用途、期限切れの場合は
// 第 2 戻り値が false になります。
func (m *Manager) GetEmailConfirmationID(r *http.Request) (model.EmailConfirmationID, bool) {
	cookie, err := r.Cookie(EmailConfirmationCookieName)
	if err != nil {
		return 0, false
	}
	id, ok := verifyContinuationToken(m.cfg.ContinuationTokenKey, emailConfirmationTokenPurpose, cookie.Value, time.Now())
	if !ok {
		return 0, false
	}
	return model.EmailConfirmationID(id), true
}

// DeleteEmailConfirmationID clears the email-confirmation cookie by setting a
// matching cookie with MaxAge < 0, so a completed or abandoned confirmation does
// not linger. The attributes mirror SetEmailConfirmationID so the browser
// matches and removes it.
//
// [Ja] DeleteEmailConfirmationID は MaxAge < 0 の同名 Cookie を設定してメール確認
// Cookie を消去し、完了または放棄された確認が残らないようにします。属性は
// SetEmailConfirmationID と揃え、ブラウザが一致して削除できるようにします。
func (m *Manager) DeleteEmailConfirmationID(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     EmailConfirmationCookieName,
		Value:    "",
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// SetTwoFactorPendingUserID writes a signed, purpose-bound continuation token for
// the pending user to its cookie. No session is issued yet: this token marks that
// the password step passed for a 2FA-enabled user, and the challenge exchanges it
// for a real session on a valid code. Its signed expiry matches MaxAge. The
// cookie is HttpOnly because it carries server-side state; the other attributes
// mirror the session cookie's policy (Secure only in production, SameSite=Lax).
//
// [Ja] SetTwoFactorPendingUserID は保留中ユーザー用に、署名され用途を結び付けた
// continuation token を専用 Cookie へ書き込みます。この時点ではまだセッションを発行せず、
// token は 2FA 有効なユーザーでパスワードのステップが通ったことを示し、正しいコードと
// 引き換えに本物のセッションへ交換されます。署名対象の有効期限は MaxAge と揃えます。
// サーバー側状態を運ぶため Cookie は HttpOnly とし、他の属性はセッション Cookie の方針
// (Secure は本番のみ・SameSite=Lax) に揃えます。
func (m *Manager) SetTwoFactorPendingUserID(w http.ResponseWriter, id model.UserID) {
	expiresAt := time.Now().Add(twoFactorPendingCookieMaxAge * time.Second)
	token := signContinuationToken(m.cfg.ContinuationTokenKey, twoFactorPendingTokenPurpose, int64(id), expiresAt)

	http.SetCookie(w, &http.Cookie{
		Name:     TwoFactorPendingCookieName,
		Value:    token,
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   twoFactorPendingCookieMaxAge,
	})
}

// GetTwoFactorPendingUserID returns the pending user id authenticated by the
// request cookie's continuation token. The second return value is false when the
// cookie is absent, malformed, forged, for another purpose, or expired.
//
// [Ja] GetTwoFactorPendingUserID はリクエスト Cookie の continuation token で認証された
// 保留中のユーザー id を返します。Cookie が無い、形式不正、偽造、別用途、期限切れの場合は
// 第 2 戻り値が false になります。
func (m *Manager) GetTwoFactorPendingUserID(r *http.Request) (model.UserID, bool) {
	cookie, err := r.Cookie(TwoFactorPendingCookieName)
	if err != nil {
		return 0, false
	}
	id, ok := verifyContinuationToken(m.cfg.ContinuationTokenKey, twoFactorPendingTokenPurpose, cookie.Value, time.Now())
	if !ok {
		return 0, false
	}
	return model.UserID(id), true
}

// DeleteTwoFactorPendingUserID clears the two-factor pending cookie by setting a
// matching cookie with MaxAge < 0, so a completed or abandoned challenge does not
// linger. The attributes mirror SetTwoFactorPendingUserID so the browser matches
// and removes it.
//
// [Ja] DeleteTwoFactorPendingUserID は MaxAge < 0 の同名 Cookie を設定して 2 段階認証の
// pending Cookie を消去し、完了または放棄されたチャレンジが残らないようにします。属性は
// SetTwoFactorPendingUserID と揃え、ブラウザが一致して削除できるようにします。
func (m *Manager) DeleteTwoFactorPendingUserID(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     TwoFactorPendingCookieName,
		Value:    "",
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// SessionToken returns the session token from the request cookie, or "" when the
// cookie is absent. Sign-out reads it to delete the matching session row before
// clearing the cookie, and GetCurrentUser reads it the same way to resolve the
// signed-in user, so this is the single source for reading the token.
//
// [Ja] SessionToken はリクエスト Cookie からセッショントークンを返します。Cookie が
// 無い場合は "" を返します。サインアウトは Cookie を消去する前に一致するセッション行を
// 削除するためにこれを読み、GetCurrentUser も同じ方法でサインイン済みユーザーを解決する
// ため、これがトークン読み取りの単一の情報源です。
func (m *Manager) SessionToken(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
