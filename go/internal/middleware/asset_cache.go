package middleware

import (
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
)

// 静的アセットに使うCache-Controlの値。
//
// 非開発環境は各アセットURLをデプロイ時のバージョン付きで配信するため、キャッシュ
// された内容が古くなることはありません。更新するとクエリが変わり、ブラウザは新しいURLを
// 取得します。開発環境は描画のたびにバージョンを作り直すため、二度と要求されない複製で
// キャッシュを埋めることになるので、何も保持させません。
//
// publicではなくprivateとするのは、アセットのレスポンスが発行したてのCSRFトークンの
// Set-Cookieを伴いうるためです。共有キャッシュに保存を許すと、ある訪問者のトークンを次の
// 訪問者へ渡してしまいます。これで手放すのは共有キャッシュだけであり、バージョン付きURLが
// 効かせたいのはブラウザのキャッシュです。
const (
	assetCacheControl          = "private, max-age=31536000, immutable"
	assetCacheControlDev       = "no-store"
	assetCacheControlNotServed = "private, no-store"
)

// AssetCacheは、静的アセットをどれだけ保持してよいかを宣言するミドルウェアを
// 返します。
//
// 明示的な保持期間はファイルの更新時刻の代わりになるものです。アセットはバイナリから
// 配信され、埋め込まれたファイルはファイルサーバーがLast-Modifiedヘッダーに変換できる
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

// assetCacheWriterは、ファイルサーバーがアセットを配信しなかったときに、長期の
// Cache-Control方針を置き換えます。ヘッダーを省くだけでは404や恒久redirectが
// キャッシュされうるため、存在しない・移動したパスは明示的に保存させません。正常なRange
// 応答は200と同じバージョン付き表現の一部を運ぶため、アセットの方針を維持します。
type assetCacheWriter struct {
	http.ResponseWriter
}

// WriteHeaderはアセット以外のステータス行が送出される前にヘッダーを置き換えます。
// ステータスを設定せず本文を書くハンドラーは200を送りここを通りませんが、それこそが
// 長期のヘッダーを付けたい場合です。
func (w *assetCacheWriter) WriteHeader(status int) {
	if status != http.StatusOK && status != http.StatusPartialContent {
		w.Header().Set("Cache-Control", assetCacheControlNotServed)
	}
	w.ResponseWriter.WriteHeader(status)
}

// Unwrapは下層のResponseWriterを返します。http.ResponseControllerが、サーバー
// 自身のResponseWriterが実装する追加のインターフェース (Flusher・Hijacker・デッド
// ラインの設定など) へ到達するための手段です。これが無いと、/static/* の下でこの
// ラッパーだけがそれらを利かなくします。
func (w *assetCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
