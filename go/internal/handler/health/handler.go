// Package health provides the handler for the health check endpoint.
//
// [Ja] health パッケージは、ヘルスチェックエンドポイントのハンドラーを提供します。
package health

// Handler is the HTTP handler for the health check endpoint.
//
// [Ja] Handler はヘルスチェックエンドポイントの HTTP ハンドラーです。
type Handler struct{}

// NewHandler creates a new health Handler.
//
// [Ja] NewHandler は新しい health Handler を作成します。
func NewHandler() *Handler {
	return &Handler{}
}
