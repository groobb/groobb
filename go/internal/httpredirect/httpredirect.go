// Package httpredirect sends the redirects that more than one resource answers
// with. It lives outside internal/handler for the same reason httperror does:
// what it answers belongs to no one resource directory, and putting it under one
// would create an exception to that directory's file-name rules.
//
// [Ja] httpredirect パッケージは、複数のリソースが返すリダイレクトを送ります。
// internal/handler の外に置くのは httperror と同じ理由で、ここが応じるものは特定の
// リソースディレクトリのものではなく、どれかの下に置けばそのファイル名の規約に例外を
// 作ることになるためです。
package httpredirect

import (
	"net/http"

	"github.com/groobb/groobb/go/internal/templates"
)

// canonicalCacheControl lets a browser hold a canonical redirect without
// letting a shared cache store it. The destination is the same for everyone,
// but the global CSRF middleware may attach a newly minted, visitor-specific
// token as Set-Cookie before the handler runs. A private lifetime keeps that
// token within the visitor's browser while avoiding needless revalidation of a
// redirect that is permanent by definition.
//
// [Ja] canonicalCacheControl は、正規 URL へのリダイレクトを共有キャッシュには保存
// させず、ブラウザには保持させるための値です。行き先は訪問者によらず同じですが、
// グローバルな CSRF ミドルウェアはハンドラーが走る前に、新しく発行した訪問者固有の
// トークンを Set-Cookie として添える場合があります。private な有効期間により、その
// トークンを訪問者のブラウザ内に留めながら、定義上恒久であるリダイレクトの不要な
// 再検証を避けます。
const canonicalCacheControl = "private, max-age=3600"

// ToCanonical sends the request to the resource's canonical URL with a permanent
// redirect, so both visitors and crawlers settle on the one address it is served
// at. It is what a route resolving a slug case-insensitively answers a request
// that reached the resource by a spelling other than the stored one.
//
// The request's query string is carried over. It is not part of what makes the
// URL non-canonical — only the spelling of the address is — so dropping it would
// take a campaign parameter, or a listing parameter, away from whoever arrived
// through a differently cased link.
//
// [Ja] ToCanonical はリクエストをそのリソースの正規 URL へ恒久リダイレクトで送り、
// 訪問者もクローラーも、それが配信される 1 つのアドレスに落ち着くようにします。slug を
// 大文字小文字を無視して解決するルートが、保存されているものと異なる綴りでリソースへ
// 到達したリクエストに返すものです。
//
// リクエストのクエリ文字列は引き継ぎます。URL を非正規にしているのはアドレスの綴りだけ
// であってクエリではないため、落としてしまうと、大文字小文字の異なるリンクから来た人
// だけが計測用のパラメータや一覧のパラメータを失うことになります。
func ToCanonical(w http.ResponseWriter, r *http.Request, path templates.Path) {
	target := path.String()
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	w.Header().Set("Cache-Control", canonicalCacheControl)
	http.Redirect(w, r, target, http.StatusPermanentRedirect)
}
