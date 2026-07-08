// Package home provides the handler for the signed-in home page (GET /home),
// the first page a user lands on after signing in.
//
// [Ja] home パッケージはサインイン済みユーザーのホームページ (GET /home) の
// ハンドラーを提供します。サインイン後に最初に着地するページです。
package home

import "github.com/groobb/groobb/go/internal/config"

// Handler is the HTTP handler for the home page.
//
// [Ja] Handler はホームページの HTTP ハンドラーです。
type Handler struct {
	cfg *config.Config
}

// NewHandler creates a new home Handler.
//
// [Ja] NewHandler は新しい home Handler を作成します。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
