// welcomeパッケージは、トップページ (GET /) のハンドラーを提供します。
package welcome

import "github.com/groobb/groobb/go/internal/config"

// HandlerはトップページのHTTPハンドラーです。
type Handler struct {
	cfg *config.Config
}

// NewHandlerは新しいwelcome Handlerを作成します。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
