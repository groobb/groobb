package middleware

import "net/http"

// redirectByName names Groobb as the software that issued a redirect. It is the
// stable product name rather than a version or a route, so that reading the
// header tells an operator which layer answered without exposing changing
// deployment details.
//
// [Ja] redirectByName は、リダイレクトを発行したソフトウェアとして Groobb を示す名前で
// ある。バージョンやルートではなく安定した製品名にするのは、運用者にどの層が応答したかを
// 伝えつつ、変動するデプロイの詳細を公開しないためである。
const redirectByName = "groobb"

// RedirectBy returns middleware that names Groobb as the issuer of every redirect
// the application writes.
//
// A production instance sits behind Cloudflare and Dokku's own proxy, and each of
// those layers can issue a redirect of its own. A 3xx by itself does not say which
// one wrote it, so tracking down a URL that bounces unexpectedly means reading
// three configurations. Naming the issuing layer in the response turns that into
// reading one header per hop.
//
// [Ja] RedirectBy は、アプリケーションが書き出すすべてのリダイレクトについて、その発行元
// として Groobb を示すミドルウェアを返す。
//
// 本番のインスタンスは Cloudflare と Dokku 内蔵のプロキシの後ろに置かれ、それらの層は
// どれも自身でリダイレクトを発行しうる。3xx だけではどの層が書いたのかが分からないため、
// 意図せず転送される URL を追うには 3 つの設定を読むことになる。発行元をレスポンスで
// 示せば、それが 1 ホップにつき 1 つのヘッダーを読むだけの作業になる。
func RedirectBy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&redirectByWriter{ResponseWriter: w}, r)
	})
}

// redirectByWriter adds the header while the status line is still unsent, which
// is why the status is caught here instead of being read after the fact: a header
// set once the status has been flushed never reaches the client.
//
// [Ja] redirectByWriter は、ステータス行がまだ送出されていない間にヘッダーを足す。事後に
// ステータスを読むのではなくここで捕まえるのはそのためである。ステータスを送出した後に
// 設定したヘッダーはクライアントに届かない。
type redirectByWriter struct {
	http.ResponseWriter
}

// WriteHeader names the issuer of a redirect, and only of a redirect. A 3xx is
// required to carry a Location because the header claims that this response sends
// the client elsewhere: 304 Not Modified is a 3xx that sends the client nowhere,
// and marking it would make the claim false. A handler that writes a body without
// setting a status sends 200 and never reaches here, which needs no header
// either.
//
// [Ja] WriteHeader はリダイレクトの発行元を示す。示すのはリダイレクトについてだけである。
// 3xx に Location を求めるのは、このヘッダーが「このレスポンスはクライアントを別の場所へ
// 送る」と主張するものだからである。304 Not Modified はクライアントをどこへも送らない
// 3xx であり、これに印を付けるとその主張が偽になる。ステータスを設定せず本文を書く
// ハンドラーは 200 を送りここを通らないが、それにもヘッダーは要らない。
func (w *redirectByWriter) WriteHeader(status int) {
	if status >= http.StatusMultipleChoices && status < http.StatusBadRequest &&
		w.Header().Get("Location") != "" {
		w.Header().Set("Redirect-By", redirectByName)
	}
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap returns the writer underneath, which is how http.ResponseController
// reaches the optional interfaces (Flusher, Hijacker, deadline setters) that the
// server's own writer implements. This wrapper covers every route, so without it
// those would stop working everywhere.
//
// [Ja] Unwrap は下層の ResponseWriter を返す。http.ResponseController が、サーバー自身の
// ResponseWriter が実装する追加のインターフェース (Flusher・Hijacker・デッドラインの
// 設定など) へ到達するための手段である。このラッパーは全ルートを覆うため、これが無いと
// それらがどこでも利かなくなる。
func (w *redirectByWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
