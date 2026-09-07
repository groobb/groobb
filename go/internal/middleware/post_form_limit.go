package middleware

import (
	"errors"
	"mime"
	"net/http"
	"strings"
)

// PostFormMaxBytes is the largest request body accepted on the routes that
// submit a post: 128 KiB.
//
// The value is derived from the lengths the fields accept, not from everything
// a submission may carry. Those fields are limited in Unicode code points, and
// one code point costs up to four bytes in UTF-8 and three characters per byte
// once the browser percent-encodes it, so a body of 10,000 code points and a
// title of 100 arrive as about 118.5 KiB with the language, the CSRF token and
// the field names beside them. The rest of the 128 KiB is the room left for what
// those lengths do not count, such as the whitespace a title is trimmed of
// before it is measured. What is bounded here is the request as it arrives,
// before any of it has been read as a field, so the limit is a size in bytes
// rather than a count derived from those per-field limits.
//
// [Ja] PostFormMaxBytes は投稿を送信するルートで受け付けるリクエストボディの最大の
// 大きさ (128KiB) です。
//
// この値はフィールドが受け付ける長さから算出したものであり、送信が運びうるすべてを
// 賄うものではありません。それらのフィールドの上限はUnicodeのコードポイント数であり、
// 1コードポイントはUTF-8で最大4バイト、ブラウザがパーセントエンコードすると1バイト
// あたり3文字になるため、10,000コードポイントの本文と100コードポイントのタイトルは、
// 主言語・CSRFトークン・フィールド名と合わせて約118.5KiBで届きます。128KiBの残りは、
// それらの長さが数えないもの (数える前に取り除かれるタイトルの前後の空白など) のための
// 余地です。ここで制限するのは、まだどの部分もフィールドとして読まれていない、届いた
// 時点のリクエストであるため、上限はフィールドごとの上限から導いた文字数ではなくバイト数
// で表します。
const PostFormMaxBytes = 128 << 10

// postFormContentType is the encoding the post forms are submitted in. They are
// ordinary HTML forms with no file upload, so this is the only encoding a
// submission from them can arrive as.
//
// [Ja] postFormContentType は投稿フォームが送信されるエンコード方式です。ファイルの
// アップロードを持たない通常のHTMLフォームであるため、そこからの送信が届きうる方式は
// これだけです。
const postFormContentType = "application/x-www-form-urlencoded"

// PostFormLimit bounds and parses the body of the two POST routes that submit a
// post, answering 413 when it exceeds PostFormMaxBytes, 415 when it is not a
// form, and 400 when the form encoding the request carries cannot be decoded.
// The decoding covers the query string as well as the body, so a request whose
// URL is encoded as badly as its body is answered the same way.
//
// It has to run ahead of everything that reads the body, which the CSRF check
// does to find the submitted token. Reaching that check first, an oversized or
// undecodable body would be indistinguishable from a missing token and would be
// answered as a rejected one; and its own read caches an empty form on the
// request, which every later reader would take for a submission with nothing in
// it. Both are answered here for what they are instead.
//
// The two routes are the only ones it applies to, so the forms that already
// reach the other routes keep the bounds they are served under today.
//
// [Ja] PostFormLimit は投稿を送信する2つのPOSTルートのボディを制限して解析し、
// PostFormMaxBytes を超えるものには413を、フォームでないものには415を、リクエストが
// 運ぶフォーム符号化をデコードできないものには400を応答します。デコードの対象はボディ
// だけでなくクエリ文字列も含むため、URLの符号化がボディと同じく壊れているリクエストも
// 同じ応答になります。
//
// ボディを読むすべてのものより前に走る必要があります。CSRF検証は送信されたトークンを
// 見つけるためにボディを読むためです。その検証に先に到達すると、大きすぎる・デコード
// できないボディはトークンが無い状態と見分けられず、拒否されたトークンとして応答されて
// しまいます。またその読み取りは空のフォームをリクエストにキャッシュし、以降の読み手は
// それを中身の無い送信として受け取ります。どちらもここでそれとして応答します。
//
// 適用するのはその2つのルートだけであるため、既に他のルートへ届いているフォームは今日
// 配信されている条件のままです。
func PostFormLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Match chi's routing input: decoding an escaped slash here would split
		// one identifier into two segments and allow the request to bypass the limit.
		//
		// [Ja] chiの照合に使うパスに揃える。ここでエンコードされたスラッシュをデコードすると、
		// 1つの識別子が2つのセグメントに分かれ、リクエストが制限を通過してしまう。
		routePath := r.URL.RawPath
		if routePath == "" {
			routePath = r.URL.Path
		}
		if r.Method != http.MethodPost || !isPostFormPath(routePath) {
			next.ServeHTTP(w, r)
			return
		}

		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != postFormContentType {
			http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
			return
		}

		// Turn away a body that announces its size as over the limit without
		// reading it. A chunked request announces none, which leaves
		// ContentLength at -1 and the decision to the reader below.
		//
		// [Ja] 上限を超える大きさを自ら申告しているボディは、読まずに追い返す。chunked の
		// リクエストは大きさを申告しないため ContentLength は -1 になり、判断は下の
		// リーダーに委ねられる。
		if r.ContentLength > PostFormMaxBytes {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, PostFormMaxBytes)

		// Parse here so the form is decoded under the limit just set, and so a
		// failure to decode it is answered rather than left for a later reader to
		// find as an empty form. The parsed values are cached on the request,
		// which is what the readers after this one, and the handler itself, go on
		// to read.
		//
		// [Ja] ここで解析するのは、今設定した上限のもとでフォームをデコードするため、
		// そしてデコードの失敗を、後の読み手が空のフォームとして受け取るのに任せず応答
		// するためである。解析した値はリクエストにキャッシュされ、これより後の読み手も
		// ハンドラー自身も、それを読むことになる。
		if err := r.ParseForm(); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isPostFormPath reports whether path addresses one of the two routes a post is
// submitted to: /b/{slug}/threads, which starts a thread, and /t/{id}/posts,
// which replies to one.
//
// The routes are named here by their shape rather than read from the router,
// because a middleware answers a request before the router has matched it: the
// pattern that will handle it is not known yet. The shape is the pair of fixed
// segments around the identifier, so a request the router goes on to turn away
// as an unknown board or thread is bounded on its way there as well.
//
// [Ja] isPostFormPath は、path が投稿を送信する2つのルート (スレッドを立てる
// /b/{slug}/threads と、スレッドに返信する /t/{id}/posts) のいずれかを指すかどうかを
// 返します。
//
// ルートをルーターから読むのではなくここでその形として名指すのは、ミドルウェアが
// リクエストに応答するのがルーターの照合より前であり、どのパターンが処理することに
// なるかがまだ分からないためです。形とは識別子を挟む2つの固定のセグメントであり、
// ルーターがこの後に存在しない掲示板・スレッドとして追い返すリクエストも、そこへ向かう
// 途中で同じように制限されます。
func isPostFormPath(path string) bool {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) != 3 || segments[1] == "" {
		return false
	}

	switch segments[0] {
	case "b":
		return segments[2] == "threads"
	case "t":
		return segments[2] == "posts"
	default:
		return false
	}
}
