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

	"github.com/google/uuid"

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

// EmailConfirmationCookieName is the name of the cookie that carries the pending
// email confirmation's id across the sign-up handoff: sign-up issues a
// confirmation and stores its id here, and the code-entry step reads it back to
// know which confirmation to verify. It carries the project prefix per the
// identifier naming convention.
//
// [Ja] EmailConfirmationCookieName は、サインアップの受け渡しをまたいで保留中のメール
// 確認の id を運ぶ Cookie の名前です。サインアップは確認を発行してその id をここに
// 保存し、コード入力のステップがこれを読み戻してどの確認を検証すべきかを知ります。
// 識別子の命名規約に従いプロジェクト接頭辞を付けています。
const EmailConfirmationCookieName = "groobb_email_confirmation_id"

// TwoFactorPendingCookieName is the name of the cookie that carries the pending
// user's id across the two-factor sign-in handoff: after the password check
// passes for a 2FA-enabled account, sign-in stores the user's id here instead of
// issuing a session, and the TOTP / recovery-code challenge reads it back to know
// whose second factor to verify. It carries the project prefix per the identifier
// naming convention.
//
// [Ja] TwoFactorPendingCookieName は、2 段階認証のサインインの受け渡しをまたいで保留中の
// ユーザーの id を運ぶ Cookie の名前です。2FA 有効なアカウントでパスワード検証が通った後、
// サインインはセッションを発行する代わりにユーザーの id をここに保存し、TOTP / リカバリー
// コードのチャレンジがこれを読み戻して誰の第 2 要素を検証すべきかを知ります。識別子の
// 命名規約に従いプロジェクト接頭辞を付けています。
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

// SetEmailConfirmationID writes the pending confirmation's id to its cookie so
// the next step (code entry) can read which confirmation to verify. The cookie
// is HttpOnly because the id is server-side state the browser only relays; the
// other attributes mirror the session cookie's policy (Secure only in
// production, SameSite=Lax).
//
// [Ja] SetEmailConfirmationID は保留中の確認の id を専用 Cookie に書き込み、次の
// ステップ (コード入力) がどの確認を検証すべきかを読めるようにします。id はブラウザが
// 中継するだけのサーバー側の状態のため Cookie は HttpOnly とし、他の属性はセッション
// Cookie の方針 (Secure は本番のみ・SameSite=Lax) に揃えます。
func (m *Manager) SetEmailConfirmationID(w http.ResponseWriter, id model.EmailConfirmationID) {
	http.SetCookie(w, &http.Cookie{
		Name:     EmailConfirmationCookieName,
		Value:    id.String(),
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   emailConfirmationCookieMaxAge,
	})
}

// GetEmailConfirmationID returns the pending confirmation id stored in the
// request cookie. The second return value is false when the cookie is absent or
// its value is not a valid UUID (e.g. a corrupt or forged cookie), so callers
// treat a malformed cookie the same as no cookie.
//
// [Ja] GetEmailConfirmationID はリクエスト Cookie に格納された保留中の確認 id を
// 返します。Cookie が無い、または値が妥当な UUID でない (壊れた / 偽造された Cookie
// など) 場合は第 2 戻り値が false になり、呼び出し側は不正な Cookie を Cookie 無しと
// 同じに扱えます。
func (m *Manager) GetEmailConfirmationID(r *http.Request) (model.EmailConfirmationID, bool) {
	cookie, err := r.Cookie(EmailConfirmationCookieName)
	if err != nil {
		return model.EmailConfirmationID{}, false
	}
	parsed, err := uuid.Parse(cookie.Value)
	if err != nil {
		return model.EmailConfirmationID{}, false
	}
	return model.EmailConfirmationID(parsed), true
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

// SetTwoFactorPendingUserID writes the pending user's id to its cookie so the
// TOTP / recovery-code challenge can read whose second factor to verify. No
// session is issued yet: this cookie only marks that the password step passed for
// a 2FA-enabled user, and the challenge exchanges it for a real session on a
// valid code. The cookie is HttpOnly because the id is server-side state the
// browser only relays; the other attributes mirror the session cookie's policy
// (Secure only in production, SameSite=Lax).
//
// [Ja] SetTwoFactorPendingUserID は保留中のユーザーの id を専用 Cookie に書き込み、
// TOTP / リカバリーコードのチャレンジが誰の第 2 要素を検証すべきかを読めるようにします。
// この時点ではまだセッションを発行しません。この Cookie は 2FA 有効なユーザーでパスワードの
// ステップが通ったことを示すだけで、チャレンジが正しいコードと引き換えに本物のセッションへ
// 交換します。id はブラウザが中継するだけのサーバー側の状態のため Cookie は HttpOnly とし、
// 他の属性はセッション Cookie の方針 (Secure は本番のみ・SameSite=Lax) に揃えます。
func (m *Manager) SetTwoFactorPendingUserID(w http.ResponseWriter, id model.UserID) {
	http.SetCookie(w, &http.Cookie{
		Name:     TwoFactorPendingCookieName,
		Value:    id.String(),
		Path:     "/",
		Secure:   m.cfg.IsProduction(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   twoFactorPendingCookieMaxAge,
	})
}

// GetTwoFactorPendingUserID returns the pending user id stored in the request
// cookie. The second return value is false when the cookie is absent or its value
// is not a valid UUID (e.g. a corrupt or forged cookie), so callers treat a
// malformed cookie the same as no cookie.
//
// [Ja] GetTwoFactorPendingUserID はリクエスト Cookie に格納された保留中のユーザー id を
// 返します。Cookie が無い、または値が妥当な UUID でない (壊れた / 偽造された Cookie など)
// 場合は第 2 戻り値が false になり、呼び出し側は不正な Cookie を Cookie 無しと同じに
// 扱えます。
func (m *Manager) GetTwoFactorPendingUserID(r *http.Request) (model.UserID, bool) {
	cookie, err := r.Cookie(TwoFactorPendingCookieName)
	if err != nil {
		return model.UserID{}, false
	}
	parsed, err := uuid.Parse(cookie.Value)
	if err != nil {
		return model.UserID{}, false
	}
	return model.UserID(parsed), true
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
