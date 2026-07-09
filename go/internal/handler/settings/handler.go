// Package settings provides the handler for the settings hub (GET /settings), the
// page that links to the individual settings screens such as email change.
//
// [Ja] settings パッケージは設定ハブ (GET /settings) のハンドラーを提供します。
// メールアドレス変更などの各設定画面へリンクするページです。
package settings

import "github.com/groobb/groobb/go/internal/config"

// Handler is the HTTP handler for the settings hub.
//
// [Ja] Handler は設定ハブの HTTP ハンドラーです。
type Handler struct {
	cfg *config.Config
}

// NewHandler creates a new settings Handler.
//
// [Ja] NewHandler は新しい settings Handler を作成します。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
