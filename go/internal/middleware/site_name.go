package middleware

import (
	"context"
	"log/slog"
	"net/http"
)

// SiteName holds the dependencies of the middleware that names the site every
// page belongs to.
//
// [Ja] SiteName は、どのページも属するサイトを名付けるミドルウェアの依存を保持する。
type SiteName struct {
	resolve func(context.Context) (string, error)
	set     func(context.Context, string) context.Context
}

// NewSiteName creates a SiteName middleware from the two seams it needs: one
// resolves the site's name, and the other stores that name in a context. The
// composition root adapts the application UseCase and the presentation context
// value to these functions, keeping this common HTTP middleware independent of
// both packages.
//
// [Ja] NewSiteName は、サイト名を解決する関数と、その名前を context に格納する関数の
// 2 つから SiteName ミドルウェアを生成します。composition root が Application 層の
// UseCase と Presentation 層の context 値をこれらの関数へ適合させることで、この共通の
// HTTP ミドルウェアはどちらのパッケージにも依存しません。
func NewSiteName(
	resolve func(context.Context) (string, error),
	set func(context.Context, string) context.Context,
) *SiteName {
	return &SiteName{resolve: resolve, set: set}
}

// Middleware resolves the community this instance hosts and stores its name in
// the request context, where the metadata of the page being rendered reads it as
// the name to end its title with. It wraps every route rather than the four
// pages that load the community for their sidebar, because the pages drawn in
// the Default layout (the sign-in form, the 404 page) load none of their own and
// their titles name the same site.
//
// The name is resolved per request rather than once at startup because the row
// is written from outside the serving process (the seed command creates it), and
// a name read once would go on naming what the community was called when the
// server started. The read is a single row addressed by its primary key.
//
// A read that fails, and an instance whose community has not been created yet,
// both leave the context without a name, and the pages then carry their own
// names alone. Answering 500 instead would turn a database that is briefly
// unreachable into an outage of the pages that need no database at all — the top
// page and the 404 — over the tail of their titles.
//
// [Ja] Middleware はこのインスタンスが運営するコミュニティを解決し、その名前をリクエスト
// context に格納する。今描画しているページのメタ情報が、そこからタイトルの末尾に置く名前を
// 読む。サイドバーのためにコミュニティを読み込む 4 ページだけでなく全ルートに掛けるのは、
// Default レイアウトで描画されるページ (サインインフォーム・404 ページ) が自前では
// コミュニティを読み込まない一方、そのタイトルが名指すサイトは同じだからである。
//
// 起動時に一度ではなくリクエストごとに解決するのは、行が配信プロセスの外から書かれる
// (シードコマンドが作る) ためで、一度だけ読んだ名前はサーバー起動時点でのコミュニティの
// 呼び名を名乗り続けることになる。読み取りは主キーで指定した 1 行である。
//
// 読み取りの失敗も、コミュニティがまだ作られていないインスタンスも、ともに context を
// 名前の無いままにし、その場合ページは自身の名前だけを運ぶ。ここで 500 を返すと、一時的に
// 到達できないデータベースを、データベースを必要としないページ (トップページと 404) まで
// 落とす障害に変えてしまう。タイトルの末尾のためにである。
func (s *SiteName) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		name, err := s.resolve(ctx)
		if err != nil {
			slog.WarnContext(ctx, "コミュニティの取得に失敗", "error", err)
			next.ServeHTTP(w, r)
			return
		}
		if name == "" {
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(s.set(ctx, name)))
	})
}
