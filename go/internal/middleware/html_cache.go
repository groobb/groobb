package middleware

import (
	"mime"
	"net/http"
)

// htmlCacheControl keeps an HTML response out of a shared cache while asking the
// browser to revalidate before reusing it. Every HTML route mints or reuses a
// visitor-specific CSRF token and renders the visitor's signed-in state, so a
// copy stored by a shared cache and handed to the next visitor would carry
// someone else's token and someone else's view of the site.
//
// [Ja] htmlCacheControl は HTML レスポンスを共有キャッシュから除外しつつ、ブラウザには
// 再利用の前に再検証させる値である。どの HTML ルートも訪問者固有の CSRF トークンを発行
// または再利用し、訪問者のサインイン状態を描画するため、共有キャッシュが保存した複製を
// 次の訪問者へ渡すと、他人のトークンと他人向けの画面を運ぶことになる。
const htmlCacheControl = "private, no-cache"

// HTMLCache returns middleware that gives HTML responses a default cache
// policy while preserving a policy chosen by a more specific handler or
// middleware.
//
// [Ja] HTMLCache は HTML レスポンスに既定のキャッシュ方針を与えつつ、より具体的な
// ハンドラーやミドルウェアが選んだ方針を維持するミドルウェアを返します。
func HTMLCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&htmlCacheWriter{ResponseWriter: w}, r)
	})
}

type htmlCacheWriter struct {
	http.ResponseWriter
}

func (w *htmlCacheWriter) WriteHeader(status int) {
	w.setDefault(nil)
	w.ResponseWriter.WriteHeader(status)
}

func (w *htmlCacheWriter) Write(body []byte) (int, error) {
	w.setDefault(body)
	return w.ResponseWriter.Write(body)
}

// setDefault marks an HTML response that has not chosen a policy of its own.
// body is the content about to be written, or nil when only the status line is
// being sent. It is read solely to recognize HTML whose type the handler left
// for net/http to detect: the detected type stays local so that net/http still
// decides the Content-Type this response carries, and the conditions match the
// ones it applies before detecting, so an encoded or empty body is left alone
// here as well.
//
// [Ja] setDefault は、自身の方針を選んでいない HTML レスポンスに印を付けます。body は
// これから書き込む内容で、ステータス行だけを送出するときは nil です。読むのは、ハンドラーが
// 型の判定を net/http へ委ねた HTML を認識するためだけです。判定した型をローカルに留めること
// で、このレスポンスが運ぶ Content-Type は引き続き net/http が決め、条件も net/http が判定の
// 前に課すものに合わせているため、エンコード済み・空の本文はここでも対象外になります。
func (w *htmlCacheWriter) setDefault(body []byte) {
	if w.Header().Get("Cache-Control") != "" {
		return
	}

	contentType := w.Header().Get("Content-Type")
	if contentType == "" && len(body) > 0 && w.Header().Get("Content-Encoding") == "" {
		contentType = http.DetectContentType(body)
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType == "text/html" {
		w.Header().Set("Cache-Control", htmlCacheControl)
	}
}

// FlushError applies the default before flushing the response, because Flush
// sends an implicit 200 when no status has been written yet.
//
// [Ja] FlushError はレスポンスを Flush する前に既定値を適用します。ステータスがまだ
// 書かれていない場合、Flush が暗黙の 200 を送出するためです。
func (w *htmlCacheWriter) FlushError() error {
	w.setDefault(nil)
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the writer underneath so http.ResponseController can still
// reach the server writer's optional interfaces.
//
// [Ja] Unwrap は下層の writer を返し、http.ResponseController がサーバーの writer の
// 追加インターフェースへ引き続き到達できるようにします。
func (w *htmlCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
