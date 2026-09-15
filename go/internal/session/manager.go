// sessionパッケージはCookieベースのDBセッションとフラッシュメッセージを
// 管理します。リクエストのセッションCookieから現在のユーザーを解決し、そのCookie
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

// CookieNameはセッショントークンを格納するCookieの名前です。環境変数 /
// 識別子の命名規約に従いプロジェクト接頭辞を付けています。
const CookieName = "groobb_session_token"

// EmailConfirmationCookieNameは、サインアップの受け渡しをまたいで保留中のメール
// 確認用の署名付きcontinuation tokenを運ぶCookieの名前です。tokenは確認id・用途・
// 有効期限を結び付け、コード入力のステップがブラウザー提供のidを信頼せず確認を特定
// できるようにします。識別子の命名規約に従いプロジェクト接頭辞を付けています。
const EmailConfirmationCookieName = "groobb_email_confirmation"

// TwoFactorPendingCookieNameは、2段階認証のサインイン受け渡しをまたいで署名付き
// continuation tokenを運ぶCookieの名前です。パスワード検証後にtokenが保留中ユーザーの
// id・用途・有効期限を結び付け、TOTP / リカバリーコードのチャレンジは誰の第2要素を調べるか
// 選ぶ前にそれを検証します。識別子の命名規約に従いプロジェクト接頭辞を付けています。
const TwoFactorPendingCookieName = "groobb_two_factor_pending"

// emailConfirmationCookieMaxAgeはメール確認Cookieの有効期間 (秒、15分) です。
// 確認コード自体の有効期限ウィンドウ (Korylusの慣行は15分) に揃え、Cookieが指す先の
// コードより長く生存しないようにします。
const emailConfirmationCookieMaxAge = 15 * 60

// twoFactorPendingCookieMaxAgeは2段階認証のpending Cookieの有効期間 (秒、10分)
// です。ウィンドウは意図的に短くしています。パスワードのステップ直後にTOTPまたは
// リカバリーコードを入力する間だけを賄えればよく、短い寿命は漏えいしたpending Cookieが
// 有用でいられる時間を抑えます。
const twoFactorPendingCookieMaxAge = 10 * 60

// sessionCookieMaxAgeはセッションCookieの有効期間 (秒) です。セッションには
// サーバー側の有効期限カラムが無く (user_sessionsの行は明示的なサインアウトまで
// 生存する) ため、Cookieは意図的に長寿命にしています (「サインアウトするまで
// ログイン状態を保つ」)。値はプロジェクト間の一貫性のため姉妹Korylusプロジェクトに
// 揃えています。
const sessionCookieMaxAge = 10 * 365 * 24 * 60 * 60

// ManagerはセッションCookieから現在のユーザーを解決し、そのCookieの
// ライフサイクルを管理します。セッションの作成自体は行いません。セッション行の
// 永続化はサインインUseCaseの責務で、その後にSetSessionCookieを呼びます。
type Manager struct {
	userRepo *repository.UserRepository
	cfg      *config.Config
}

// NewManagerはManagerを生成します。
func NewManager(
	userRepo *repository.UserRepository,
	cfg *config.Config,
) *Manager {
	return &Manager{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// GetCurrentUserはリクエストのセッションCookieをサインイン済みユーザーに
// 解決します。未サインインのとき (Cookieが無い / tokenが未知・失効) は (nil, nil)
// を返します。非nilのエラーは本物の失敗 (例: データベースに到達できない) のために
// のみ用います。
func (m *Manager) GetCurrentUser(ctx context.Context, r *http.Request) (*model.User, error) {
	token := m.SessionToken(r)
	if token == "" {
		return nil, nil
	}

	return m.userRepo.FindBySessionToken(ctx, token)
}

// SetSessionCookieはセッショントークンをセッションCookieに書き込みます。
// Secureは本番でのみ有効にし、dev / testでは平文HTTPでもCookieが機能する
// ようにします。HttpOnlyでJavaScriptから触れないようにし、SameSite=Laxで
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

// DeleteSessionCookieはMaxAge < 0の同名Cookieを設定してセッションCookieを
// 消去します (ブラウザに削除を指示する)。他の属性はSetSessionCookieと揃え、
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

// SetEmailConfirmationIDは保留中の確認用に、署名され用途を結び付けたcontinuation
// tokenを専用Cookieへ書き込みます。署名対象の有効期限をMaxAgeと揃え、ブラウザーだけで
// なくサーバー側でも期限を強制します。サーバー側状態を運ぶためCookieはHttpOnlyとし、
// 他の属性はセッションCookieの方針 (Secureは本番のみ・SameSite=Lax) に揃えます。
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

// GetEmailConfirmationIDはリクエストCookieのcontinuation tokenで認証された
// 保留中の確認idを返します。Cookieが無い、形式不正、偽造、別用途、期限切れの場合は
// 第2戻り値がfalseになります。
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

// DeleteEmailConfirmationIDはMaxAge < 0の同名Cookieを設定してメール確認
// Cookieを消去し、完了または放棄された確認が残らないようにします。属性は
// SetEmailConfirmationIDと揃え、ブラウザが一致して削除できるようにします。
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

// SetTwoFactorPendingUserIDは保留中ユーザー用に、署名され用途を結び付けた
// continuation tokenを専用Cookieへ書き込みます。この時点ではまだセッションを発行せず、
// tokenは2FA有効なユーザーでパスワードのステップが通ったことを示し、正しいコードと
// 引き換えに本物のセッションへ交換されます。署名対象の有効期限はMaxAgeと揃えます。
// サーバー側状態を運ぶためCookieはHttpOnlyとし、他の属性はセッションCookieの方針
// (Secureは本番のみ・SameSite=Lax) に揃えます。
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

// GetTwoFactorPendingUserIDはリクエストCookieのcontinuation tokenで認証された
// 保留中のユーザーidを返します。Cookieが無い、形式不正、偽造、別用途、期限切れの場合は
// 第2戻り値がfalseになります。
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

// DeleteTwoFactorPendingUserIDはMaxAge < 0の同名Cookieを設定して2段階認証の
// pending Cookieを消去し、完了または放棄されたチャレンジが残らないようにします。属性は
// SetTwoFactorPendingUserIDと揃え、ブラウザが一致して削除できるようにします。
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

// SessionTokenはリクエストCookieからセッショントークンを返します。Cookieが
// 無い場合は "" を返します。サインアウトはCookieを消去する前に一致するセッション行を
// 削除するためにこれを読み、GetCurrentUserも同じ方法でサインイン済みユーザーを解決する
// ため、これがトークン読み取りの単一の情報源です。
func (m *Manager) SessionToken(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
