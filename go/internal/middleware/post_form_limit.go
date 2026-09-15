package middleware

import (
	"errors"
	"mime"
	"net/http"
	"strings"
)

// PostFormMaxBytesは投稿を送信するルートで受け付けるリクエストボディの最大の
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

// postFormContentTypeは投稿フォームが送信されるエンコード方式です。ファイルの
// アップロードを持たない通常のHTMLフォームであるため、そこからの送信が届きうる方式は
// これだけです。
const postFormContentType = "application/x-www-form-urlencoded"

// PostFormLimitは投稿を送信する2つのPOSTルートのボディを制限して解析し、
// PostFormMaxBytesを超えるものには413を、フォームでないものには415を、リクエストが
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
		// chiの照合に使うパスに揃える。ここでエンコードされたスラッシュをデコードすると、
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

		// 上限を超える大きさを自ら申告しているボディは、読まずに追い返す。chunkedの
		// リクエストは大きさを申告しないためContentLengthは -1になり、判断は下の
		// リーダーに委ねられる。
		if r.ContentLength > PostFormMaxBytes {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, PostFormMaxBytes)

		// ここで解析するのは、今設定した上限のもとでフォームをデコードするため、
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

// isPostFormPathは、pathが投稿を送信する2つのルート (スレッドを立てる
// /b/{slug}/threadsと、スレッドに返信する /t/{id}/posts) のいずれかを指すかどうかを
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
