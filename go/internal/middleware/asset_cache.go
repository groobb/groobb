package middleware

import (
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
)

// Cache-Control values for the static assets.
//
// Non-dev instances serve each asset URL with the version it was deployed with,
// so a cached copy can never go stale: an upgrade changes the query and the
// browser fetches a new URL. Dev regenerates the version on every render, which
// would fill the cache with copies that are never requested again, so nothing is
// kept there.
//
// The lifetime is private rather than public because an asset response can still
// carry the Set-Cookie of a freshly minted CSRF token: a shared cache allowed to
// store it would hand one visitor's token to the next. Only a shared cache is
// given up by this, and the browser cache is what a fingerprinted URL is for.
//
// [Ja] 静的アセットに使う Cache-Control の値。
//
// 非開発環境は各アセット URL をデプロイ時のバージョン付きで配信するため、キャッシュ
// された内容が古くなることはありません。更新するとクエリが変わり、ブラウザは新しい URL を
// 取得します。開発環境は描画のたびにバージョンを作り直すため、二度と要求されない複製で
// キャッシュを埋めることになるので、何も保持させません。
//
// public ではなく private とするのは、アセットのレスポンスが発行したての CSRF トークンの
// Set-Cookie を伴いうるためです。共有キャッシュに保存を許すと、ある訪問者のトークンを次の
// 訪問者へ渡してしまいます。これで手放すのは共有キャッシュだけであり、バージョン付き URL が
// 効かせたいのはブラウザのキャッシュです。
const (
	assetCacheControl          = "private, max-age=31536000, immutable"
	assetCacheControlDev       = "no-store"
	assetCacheControlNotServed = "private, no-store"
)

// AssetCache returns middleware that declares how long the static assets may be
// kept.
//
// An explicit lifetime is what replaces the file modification time: the assets
// are served from the binary, and an embedded file has no modification time for
// the file server to turn into a Last-Modified header, so without this every
// visit would re-download them in full.
//
// [Ja] AssetCache は、静的アセットをどれだけ保持してよいかを宣言するミドルウェアを
// 返します。
//
// 明示的な保持期間はファイルの更新時刻の代わりになるものです。アセットはバイナリから
// 配信され、埋め込まれたファイルはファイルサーバーが Last-Modified ヘッダーに変換できる
// 更新時刻を持たないため、これが無いと訪問のたびに全体を再取得することになります。
func AssetCache(cfg *config.Config) func(http.Handler) http.Handler {
	value := assetCacheControl
	if cfg.IsDev() {
		value = assetCacheControlDev
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", value)
			next.ServeHTTP(&assetCacheWriter{ResponseWriter: w}, r)
		})
	}
}

// assetCacheWriter replaces the long-lived Cache-Control policy when the file
// server does not serve an asset. A missing or moved path is explicitly not
// stored, because merely omitting the header would still allow a 404 or permanent
// redirect to be cached. Successful range responses retain the asset policy
// because they carry part of the same versioned representation as a 200.
//
// [Ja] assetCacheWriter は、ファイルサーバーがアセットを配信しなかったときに、長期の
// Cache-Control 方針を置き換えます。ヘッダーを省くだけでは 404 や恒久 redirect が
// キャッシュされうるため、存在しない・移動したパスは明示的に保存させません。正常な Range
// 応答は 200 と同じバージョン付き表現の一部を運ぶため、アセットの方針を維持します。
type assetCacheWriter struct {
	http.ResponseWriter
}

// WriteHeader replaces the header before a non-asset status line is flushed. A
// handler that writes a body without setting a status sends 200 and never reaches
// here, which is exactly the case the long-lived header is meant for.
//
// [Ja] WriteHeader はアセット以外のステータス行が送出される前にヘッダーを置き換えます。
// ステータスを設定せず本文を書くハンドラーは 200 を送りここを通りませんが、それこそが
// 長期のヘッダーを付けたい場合です。
func (w *assetCacheWriter) WriteHeader(status int) {
	if status != http.StatusOK && status != http.StatusPartialContent {
		w.Header().Set("Cache-Control", assetCacheControlNotServed)
	}
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap returns the writer underneath, which is how http.ResponseController
// reaches the optional interfaces (Flusher, Hijacker, deadline setters) that the
// server's own writer implements. Without it this wrapper would be the one place
// under /static/* where those stop working.
//
// [Ja] Unwrap は下層の ResponseWriter を返します。http.ResponseController が、サーバー
// 自身の ResponseWriter が実装する追加のインターフェース (Flusher・Hijacker・デッド
// ラインの設定など) へ到達するための手段です。これが無いと、/static/* の下でこの
// ラッパーだけがそれらを利かなくします。
func (w *assetCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
