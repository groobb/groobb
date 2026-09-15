// httpredirectパッケージは、複数のリソースが返すリダイレクトを送ります。
// internal/handlerの外に置くのはhttperrorと同じ理由で、ここが応じるものは特定の
// リソースディレクトリのものではなく、どれかの下に置けばそのファイル名の規約に例外を
// 作ることになるためです。
package httpredirect

import (
	"net/http"

	"github.com/groobb/groobb/go/internal/templates"
)

// canonicalCacheControlは、正規URLへのリダイレクトを共有キャッシュには保存
// させず、ブラウザには保持させるための値です。行き先は訪問者によらず同じですが、
// グローバルなCSRFミドルウェアはハンドラーが走る前に、新しく発行した訪問者固有の
// トークンをSet-Cookieとして添える場合があります。privateな有効期間により、その
// トークンを訪問者のブラウザ内に留めながら、定義上恒久であるリダイレクトの不要な
// 再検証を避けます。
const canonicalCacheControl = "private, max-age=3600"

// ToCanonicalはリクエストをそのリソースの正規URLへ恒久リダイレクトで送り、
// 訪問者もクローラーも、それが配信される1つのアドレスに落ち着くようにします。slugを
// 大文字小文字を無視して解決するルートが、保存されているものと異なる綴りでリソースへ
// 到達したリクエストに返すものです。
//
// リクエストのクエリ文字列は引き継ぎます。URLを非正規にしているのはアドレスの綴りだけ
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
