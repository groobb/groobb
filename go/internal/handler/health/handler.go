// healthパッケージは、ヘルスチェックエンドポイントのハンドラーを提供します。
package health

// HandlerはヘルスチェックエンドポイントのHTTPハンドラーです。
type Handler struct{}

// NewHandlerは新しいhealth Handlerを作成します。
func NewHandler() *Handler {
	return &Handler{}
}
