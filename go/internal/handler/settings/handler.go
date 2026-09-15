// settingsパッケージは設定ハブ (GET /settings) のハンドラーを提供します。
// メールアドレス変更などの各設定画面へリンクするページです。
package settings

import "github.com/groobb/groobb/go/internal/config"

// Handlerは設定ハブのHTTPハンドラーです。
type Handler struct {
	cfg *config.Config
}

// NewHandlerは新しいsettings Handlerを作成します。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
