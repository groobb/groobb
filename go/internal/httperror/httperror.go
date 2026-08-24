// Package httperror renders the HTTP error responses shared by every resource.
// It lives outside internal/handler because the pages it serves answer requests
// that no resource directory owns, and putting them under a handler directory
// would create an exception to that directory's file-name rules.
//
// [Ja] httperror パッケージは全リソース共通の HTTP エラーレスポンスを描画します。
// ここが応じるページは、どのリソースディレクトリも持たないリクエストへの応答であり、
// ハンドラーのディレクトリに置くとそのファイル名の規約に例外を作ることになるため、
// internal/handler の外に置いています。
package httperror

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	errorpages "github.com/groobb/groobb/go/internal/templates/pages/errors"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Renderer renders the shared error pages. It holds the configuration for the
// same reason every page handler does: the shared layout's <head> references
// the static assets by the current asset version.
//
// [Ja] Renderer は共通のエラーページを描画します。設定を保持する理由は各ページの
// ハンドラーと同じで、共通レイアウトの <head> が現在のアセットバージョンで静的
// アセットを参照するためです。
type Renderer struct {
	cfg *config.Config
}

// NewRenderer creates a new error page Renderer.
//
// [Ja] NewRenderer は新しいエラーページの Renderer を作成します。
func NewRenderer(cfg *config.Config) *Renderer {
	return &Renderer{cfg: cfg}
}

// NotFound responds with the 404 page. It is registered as the router's
// not-found handler, so it answers every request that matches no route; the
// chi default it replaces is a bare line of plain text with no way on from it.
//
// The page is built into a buffer before anything reaches w, so a failed render
// can still answer with a plain-text 404. Rendering straight into w would have
// sent 200 and a partial body by the time the failure surfaced, leaving a
// missing page reported to crawlers and clients as a success.
//
// [Ja] NotFound は 404 ページを応答します。ルーターの not-found ハンドラーとして
// 登録し、どのルートにも一致しないすべてのリクエストに応じます。置き換える chi の
// 既定は、そこから先へ進む手段の無い平文 1 行です。
//
// ページは w へ何かが届く前にバッファ上で組み立てます。描画に失敗しても平文の 404 で
// 応答できるようにするためです。w へ直接描画すると、失敗が表面化した時点で 200 と
// 途中までのボディを送信済みであり、存在しないページをクローラーやクライアントへ
// 成功として伝えてしまいます。
func (rd *Renderer) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Declare the policy explicitly rather than leaving a 404 to the caches'
	// heuristics: a page added later at this URL must not be shadowed by a
	// stored 404. It is private because the response can carry the Set-Cookie of
	// a freshly minted CSRF token, which a shared cache must not hand on.
	//
	// [Ja] 404 をキャッシュのヒューリスティクスに委ねず、方針を明示します。この URL に
	// 後からページを追加したとき、保存された 404 がそれを覆い隠さないようにするためです。
	// private とするのは、発行したての CSRF トークンの Set-Cookie を伴いうるレスポンス
	// であり、共有キャッシュが次の訪問者へ渡してはならないためです。
	w.Header().Set("Cache-Control", "private, no-store")

	meta := viewmodel.DefaultPageMeta(ctx, rd.cfg)
	meta.Title = i18n.T(ctx, "error_not_found_title")
	meta.Description = i18n.T(ctx, "error_not_found_message")

	var body bytes.Buffer
	if err := layouts.Default(meta, errorpages.NotFound()).Render(ctx, &body); err != nil {
		slog.ErrorContext(ctx, "404 ページのレンダリングに失敗", "error", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.ErrorContext(ctx, "404 レスポンスの書き込みに失敗", "error", err)
	}
}
