package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
)

// FlashCookieNameはリダイレクトをまたいでフラッシュメッセージを運ぶCookieの
// 名前です。識別子の命名規約に従いプロジェクト接頭辞を付けています。
const FlashCookieName = "groobb_flash"

// FlashTypeはフラッシュメッセージを分類し、UIが見た目を変えられる (例: 成功
// またはエラーのtoast) ようにします。
type FlashType string

const (
	// FlashSuccessは成功メッセージを表します。
	FlashSuccess FlashType = "success"
	// FlashErrorはエラーメッセージを表します。
	FlashError FlashType = "error"
	// FlashWarningは警告メッセージを表します。
	FlashWarning FlashType = "warning"
	// FlashInfoは情報メッセージを表します。
	FlashInfo FlashType = "info"
)

// FlashMessageはフラッシュCookieに保存される単一のフラッシュメッセージです。
type FlashMessage struct {
	Type    FlashType `json:"type"`
	Message string    `json:"message"`
}

// FlashManagerは短命なCookie経由でフラッシュメッセージを設定・読み取りします。
// セッションCookieと異なりHttpOnlyではないため、クライアント側スクリプトが
// メッセージを描画 (例: toast表示) できます。
type FlashManager struct {
	cfg *config.Config
}

// NewFlashManagerはFlashManagerを生成します。
func NewFlashManager(cfg *config.Config) *FlashManager {
	return &FlashManager{cfg: cfg}
}

// SetSuccessは成功フラッシュメッセージを設定します。
func (f *FlashManager) SetSuccess(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashSuccess, message)
}

// SetErrorはエラーフラッシュメッセージを設定します。
func (f *FlashManager) SetError(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashError, message)
}

// SetWarningは警告フラッシュメッセージを設定します。
func (f *FlashManager) SetWarning(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashWarning, message)
}

// SetInfoは情報フラッシュメッセージを設定します。
func (f *FlashManager) SetInfo(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashInfo, message)
}

// setFlashはフラッシュメッセージをCookieに保存します。生のJSONにはCookie
// 値として不正な文字 (ダブルクォートなど) が含まれるため、JSONをbase64エンコード
// します。
func (f *FlashManager) setFlash(w http.ResponseWriter, flashType FlashType, message string) {
	data, err := json.Marshal(FlashMessage{Type: flashType, Message: message})
	if err != nil {
		slog.Warn("フラッシュメッセージのJSONマーシャルに失敗", "error", err)
		return
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	http.SetCookie(w, &http.Cookie{
		Name:   FlashCookieName,
		Value:  encoded,
		Path:   "/",
		Secure: f.cfg.IsProduction(),
		// toast描画のためJavaScriptから参照可能にする
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetFlashはフラッシュメッセージを読み取り、一度だけ表示されるようCookieを
// 消去します。フラッシュが無いとき、またはCookie値が壊れているときはnilを
// 返します (壊れたCookieはその場で消去します)。
func (f *FlashManager) GetFlash(w http.ResponseWriter, r *http.Request) *FlashMessage {
	cookie, err := r.Cookie(FlashCookieName)
	if err != nil {
		return nil
	}

	data, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		f.clearFlash(w)
		return nil
	}

	var flash FlashMessage
	if err := json.Unmarshal(data, &flash); err != nil {
		f.clearFlash(w)
		return nil
	}

	f.clearFlash(w)
	return &flash
}

type flashContextKey struct{}

// Middlewareはリクエストからフラッシュメッセージを読み取り、ハンドラーや
// テンプレートのためにcontextへ格納し、一度だけ表示されるようCookieを消去します。
func (f *FlashManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flash := f.GetFlash(w, r)
		if flash != nil {
			ctx := context.WithValue(r.Context(), flashContextKey{}, flash)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// FlashFromContextはMiddlewareが格納したフラッシュメッセージを返します。
// 無い場合はnilを返します。
func FlashFromContext(ctx context.Context) *FlashMessage {
	flash, _ := ctx.Value(flashContextKey{}).(*FlashMessage)
	return flash
}

// clearFlashはMaxAge < 0の同名Cookieを設定してフラッシュCookieを
// 削除します。
func (f *FlashManager) clearFlash(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     FlashCookieName,
		Value:    "",
		Path:     "/",
		Secure:   f.cfg.IsProduction(),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
