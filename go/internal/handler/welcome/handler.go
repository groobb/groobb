// Package welcome provides the handler for the top page (GET /).
//
// [Ja] welcome パッケージは、トップページ (GET /) のハンドラーを提供します。
package welcome

import "github.com/groobb/groobb/go/internal/config"

// Handler is the HTTP handler for the top page.
//
// [Ja] Handler はトップページの HTTP ハンドラーです。
type Handler struct {
	cfg *config.Config
}

// NewHandler creates a new welcome Handler.
//
// [Ja] NewHandler は新しい welcome Handler を作成します。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
