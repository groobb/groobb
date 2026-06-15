package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
)

// FlashCookieName is the name of the cookie that carries a flash message across
// a redirect. It carries the project prefix per the identifier naming
// convention.
//
// [Ja] FlashCookieName はリダイレクトをまたいでフラッシュメッセージを運ぶ Cookie の
// 名前です。識別子の命名規約に従いプロジェクト接頭辞を付けています。
const FlashCookieName = "groobb_flash"

// FlashType classifies a flash message so the UI can style it (e.g. as a success
// or error toast).
//
// [Ja] FlashType はフラッシュメッセージを分類し、UI が見た目を変えられる (例: 成功
// またはエラーの toast) ようにします。
type FlashType string

const (
	// FlashSuccess marks a success message.
	//
	// [Ja] FlashSuccess は成功メッセージを表します。
	FlashSuccess FlashType = "success"
	// FlashError marks an error message.
	//
	// [Ja] FlashError はエラーメッセージを表します。
	FlashError FlashType = "error"
	// FlashWarning marks a warning message.
	//
	// [Ja] FlashWarning は警告メッセージを表します。
	FlashWarning FlashType = "warning"
	// FlashInfo marks an informational message.
	//
	// [Ja] FlashInfo は情報メッセージを表します。
	FlashInfo FlashType = "info"
)

// FlashMessage is a single flash message persisted in the flash cookie.
//
// [Ja] FlashMessage はフラッシュ Cookie に保存される単一のフラッシュメッセージです。
type FlashMessage struct {
	Type    FlashType `json:"type"`
	Message string    `json:"message"`
}

// FlashManager sets and reads flash messages via a short-lived cookie. Unlike the
// session cookie it is not HttpOnly, so client-side scripts can render the
// message (e.g. as a toast).
//
// [Ja] FlashManager は短命な Cookie 経由でフラッシュメッセージを設定・読み取りします。
// セッション Cookie と異なり HttpOnly ではないため、クライアント側スクリプトが
// メッセージを描画 (例: toast 表示) できます。
type FlashManager struct {
	cfg *config.Config
}

// NewFlashManager creates a FlashManager.
//
// [Ja] NewFlashManager は FlashManager を生成します。
func NewFlashManager(cfg *config.Config) *FlashManager {
	return &FlashManager{cfg: cfg}
}

// SetSuccess sets a success flash message.
//
// [Ja] SetSuccess は成功フラッシュメッセージを設定します。
func (f *FlashManager) SetSuccess(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashSuccess, message)
}

// SetError sets an error flash message.
//
// [Ja] SetError はエラーフラッシュメッセージを設定します。
func (f *FlashManager) SetError(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashError, message)
}

// SetWarning sets a warning flash message.
//
// [Ja] SetWarning は警告フラッシュメッセージを設定します。
func (f *FlashManager) SetWarning(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashWarning, message)
}

// SetInfo sets an informational flash message.
//
// [Ja] SetInfo は情報フラッシュメッセージを設定します。
func (f *FlashManager) SetInfo(w http.ResponseWriter, message string) {
	f.setFlash(w, FlashInfo, message)
}

// setFlash stores the flash message in the cookie. The JSON is base64-encoded
// because raw JSON contains characters (such as double quotes) that are invalid
// in a cookie value.
//
// [Ja] setFlash はフラッシュメッセージを Cookie に保存します。生の JSON には Cookie
// 値として不正な文字 (ダブルクォートなど) が含まれるため、JSON を base64 エンコード
// します。
func (f *FlashManager) setFlash(w http.ResponseWriter, flashType FlashType, message string) {
	data, err := json.Marshal(FlashMessage{Type: flashType, Message: message})
	if err != nil {
		slog.Warn("フラッシュメッセージの JSON マーシャルに失敗", "error", err)
		return
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	http.SetCookie(w, &http.Cookie{
		Name:   FlashCookieName,
		Value:  encoded,
		Path:   "/",
		Secure: f.cfg.IsProduction(),
		// readable from JavaScript for toast rendering.
		//
		// [Ja] toast 描画のため JavaScript から参照可能にする
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetFlash reads the flash message and clears the cookie so it shows only once.
// It returns nil when there is no flash, or when the cookie value is corrupt
// (in which case the corrupt cookie is cleared).
//
// [Ja] GetFlash はフラッシュメッセージを読み取り、一度だけ表示されるよう Cookie を
// 消去します。フラッシュが無いとき、または Cookie 値が壊れているときは nil を
// 返します (壊れた Cookie はその場で消去します)。
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

// Middleware reads any flash message from the request, stores it in the context
// for handlers and templates, and clears the cookie so it is shown only once.
//
// [Ja] Middleware はリクエストからフラッシュメッセージを読み取り、ハンドラーや
// テンプレートのために context へ格納し、一度だけ表示されるよう Cookie を消去します。
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

// FlashFromContext returns the flash message stored by Middleware, or nil when
// there is none.
//
// [Ja] FlashFromContext は Middleware が格納したフラッシュメッセージを返します。
// 無い場合は nil を返します。
func FlashFromContext(ctx context.Context) *FlashMessage {
	flash, _ := ctx.Value(flashContextKey{}).(*FlashMessage)
	return flash
}

// clearFlash deletes the flash cookie by setting a matching cookie with
// MaxAge < 0.
//
// [Ja] clearFlash は MaxAge < 0 の同名 Cookie を設定してフラッシュ Cookie を
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
