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

// Forbidden responds with the 403 page. A page handler calls it when a UseCase
// refuses the request for want of permission, so that every screen answers a
// refusal with the same page instead of each writing its own.
//
// The page is built into a buffer before anything reaches w, for the reason
// NotFound documents: a failed render can then still answer with a plain-text
// 403 rather than a 200 carrying half a page.
//
// [Ja] Forbidden は 403 ページを応答します。UseCase が権限不足を理由に要求を拒んだとき
// にページのハンドラーが呼び、どの画面でも拒否が同じページで応答されるようにします。
// 各画面がそれぞれ自前のものを書かずに済みます。
//
// ページは w へ何かが届く前にバッファ上で組み立てます。理由は NotFound に記したとおりで、
// 描画に失敗しても、途中までのページを載せた 200 ではなく平文の 403 で応答できるように
// するためです。
func (rd *Renderer) Forbidden(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// A refusal is about who is asking, so it is not an answer to be handed on to
	// anyone else. no-store rather than a revalidation policy, because the same
	// URL answers with the page itself once the visitor holds the role, and a
	// stored refusal would stand in front of it.
	//
	// [Ja] 拒否は誰が尋ねているかについての応答であり、他の誰かへ渡してよい答えでは
	// ありません。再検証の方針ではなく no-store とするのは、訪問者がロールを持てば同じ URL が
	// ページ自身で応答するためです。保存された拒否はその手前に立ってしまいます。
	w.Header().Set("Cache-Control", "private, no-store")

	meta := viewmodel.DefaultPageMeta(ctx, rd.cfg)
	meta.Title = i18n.T(ctx, "error_forbidden_title")
	meta.Description = i18n.T(ctx, "error_forbidden_message")
	// The refusal carries the address of a screen that exists, so it is a page a
	// crawler could otherwise record. Nothing on it is worth finding, and it is
	// the same page whoever is refused.
	//
	// [Ja] 拒否は実在する画面のアドレスを伴うため、そうしなければクローラーが記録しうる
	// ページです。ここに見つける価値のあるものは無く、誰が拒まれても同じページです。
	meta.NoIndex = true

	var body bytes.Buffer
	if err := layouts.Default(meta, errorpages.Forbidden()).Render(ctx, &body); err != nil {
		slog.ErrorContext(ctx, "403 ページのレンダリングに失敗", "error", err)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.ErrorContext(ctx, "403 レスポンスの書き込みに失敗", "error", err)
	}
}
