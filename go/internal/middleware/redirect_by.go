package middleware

import "net/http"

// redirectByNameは、リダイレクトを発行したソフトウェアとしてGroobbを示す名前で
// ある。バージョンやルートではなく安定した製品名にするのは、運用者にどの層が応答したかを
// 伝えつつ、変動するデプロイの詳細を公開しないためである。
const redirectByName = "groobb"

// RedirectByは、アプリケーションが書き出すすべてのリダイレクトについて、その発行元
// としてGroobbを示すミドルウェアを返す。
//
// 本番のインスタンスはCloudflareとDokku内蔵のプロキシの後ろに置かれ、それらの層は
// どれも自身でリダイレクトを発行しうる。3xxだけではどの層が書いたのかが分からないため、
// 意図せず転送されるURLを追うには3つの設定を読むことになる。発行元をレスポンスで
// 示せば、それが1ホップにつき1つのヘッダーを読むだけの作業になる。
func RedirectBy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&redirectByWriter{ResponseWriter: w}, r)
	})
}

// redirectByWriterは、ステータス行がまだ送出されていない間にヘッダーを足す。事後に
// ステータスを読むのではなくここで捕まえるのはそのためである。ステータスを送出した後に
// 設定したヘッダーはクライアントに届かない。
type redirectByWriter struct {
	http.ResponseWriter
}

// WriteHeaderはリダイレクトの発行元を示す。示すのはリダイレクトについてだけである。
// 3xxにLocationを求めるのは、このヘッダーが「このレスポンスはクライアントを別の場所へ
// 送る」と主張するものだからである。304 Not Modifiedはクライアントをどこへも送らない
// 3xxであり、これに印を付けるとその主張が偽になる。ステータスを設定せず本文を書く
// ハンドラーは200を送りここを通らないが、それにもヘッダーは要らない。
func (w *redirectByWriter) WriteHeader(status int) {
	if status >= http.StatusMultipleChoices && status < http.StatusBadRequest &&
		w.Header().Get("Location") != "" {
		w.Header().Set("Redirect-By", redirectByName)
	}
	w.ResponseWriter.WriteHeader(status)
}

// Unwrapは下層のResponseWriterを返す。http.ResponseControllerが、サーバー自身の
// ResponseWriterが実装する追加のインターフェース (Flusher・Hijacker・デッドラインの
// 設定など) へ到達するための手段である。このラッパーは全ルートを覆うため、これが無いと
// それらがどこでも利かなくなる。
func (w *redirectByWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
