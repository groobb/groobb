package middleware

import (
	"context"
	"log/slog"
	"net/http"
)

// SiteNameは、どのページも属するサイトを名付けるミドルウェアの依存を保持する。
type SiteName struct {
	resolve func(context.Context) (string, error)
	set     func(context.Context, string) context.Context
}

// NewSiteNameは、サイト名を解決する関数と、その名前をcontextに格納する関数の
// 2つからSiteNameミドルウェアを生成します。composition rootがApplication層の
// UseCaseとPresentation層のcontext値をこれらの関数へ適合させることで、この共通の
// HTTPミドルウェアはどちらのパッケージにも依存しません。
func NewSiteName(
	resolve func(context.Context) (string, error),
	set func(context.Context, string) context.Context,
) *SiteName {
	return &SiteName{resolve: resolve, set: set}
}

// Middlewareはこのインスタンスが運営するコミュニティを解決し、その名前をリクエスト
// contextに格納する。今描画しているページのメタ情報が、そこからタイトルの末尾に置く名前を
// 読む。サイドバーのためにコミュニティを読み込む4ページだけでなく全ルートに掛けるのは、
// Defaultレイアウトで描画されるページ (サインインフォーム・404ページ) が自前では
// コミュニティを読み込まない一方、そのタイトルが名指すサイトは同じだからである。
//
// 起動時に一度ではなくリクエストごとに解決するのは、行が配信プロセスの外から書かれる
// (シードコマンドが作る) ためで、一度だけ読んだ名前はサーバー起動時点でのコミュニティの
// 呼び名を名乗り続けることになる。読み取りは主キーで指定した1行である。
//
// 読み取りの失敗も、コミュニティがまだ作られていないインスタンスも、ともにcontextを
// 名前の無いままにし、その場合ページは自身の名前だけを運ぶ。ここで500を返すと、一時的に
// 到達できないデータベースを、データベースを必要としないページ (トップページと404) まで
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
