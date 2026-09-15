package middleware

import (
	"mime"
	"net/http"
)

// htmlCacheControlはHTMLレスポンスを共有キャッシュから除外しつつ、ブラウザには
// 再利用の前に再検証させる値である。どのHTMLルートも訪問者固有のCSRFトークンを発行
// または再利用し、訪問者のサインイン状態を描画するため、共有キャッシュが保存した複製を
// 次の訪問者へ渡すと、他人のトークンと他人向けの画面を運ぶことになる。
const htmlCacheControl = "private, no-cache"

// HTMLCacheはHTMLレスポンスに既定のキャッシュ方針を与えつつ、より具体的な
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

// setDefaultは、自身の方針を選んでいないHTMLレスポンスに印を付けます。bodyは
// これから書き込む内容で、ステータス行だけを送出するときはnilです。読むのは、ハンドラーが
// 型の判定をnet/httpへ委ねたHTMLを認識するためだけです。判定した型をローカルに留めること
// で、このレスポンスが運ぶContent-Typeは引き続きnet/httpが決め、条件もnet/httpが判定の
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

// FlushErrorはレスポンスをFlushする前に既定値を適用します。ステータスがまだ
// 書かれていない場合、Flushが暗黙の200を送出するためです。
func (w *htmlCacheWriter) FlushError() error {
	w.setDefault(nil)
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrapは下層のwriterを返し、http.ResponseControllerがサーバーのwriterの
// 追加インターフェースへ引き続き到達できるようにします。
func (w *htmlCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
