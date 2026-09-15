// httperrorパッケージは全リソース共通のHTTPエラーレスポンスを描画します。
// ここが応じるページは、どのリソースディレクトリも持たないリクエストへの応答であり、
// ハンドラーのディレクトリに置くとそのファイル名の規約に例外を作ることになるため、
// internal/handlerの外に置いています。
package httperror

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	errorpages "github.com/groobb/groobb/go/internal/templates/pages/errors"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Rendererは共通のエラーページを描画します。設定を保持する理由は各ページの
// ハンドラーと同じで、共通レイアウトの <head> が現在のアセットバージョンで静的
// アセットを参照するためです。
type Renderer struct {
	cfg *config.Config
}

// NewRendererは新しいエラーページのRendererを作成します。
func NewRenderer(cfg *config.Config) *Renderer {
	return &Renderer{cfg: cfg}
}

// NotFoundは404ページを応答します。ルーターのnot-foundハンドラーとして
// 登録し、どのルートにも一致しないすべてのリクエストに応じます。置き換えるchiの
// 既定は、そこから先へ進む手段の無い平文1行です。
//
// ページはwへ何かが届く前にバッファ上で組み立てます。描画に失敗しても平文の404で
// 応答できるようにするためです。wへ直接描画すると、失敗が表面化した時点で200と
// 途中までのボディを送信済みであり、存在しないページをクローラーやクライアントへ
// 成功として伝えてしまいます。
func (rd *Renderer) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 404をキャッシュのヒューリスティクスに委ねず、方針を明示します。このURLに
	// 後からページを追加したとき、保存された404がそれを覆い隠さないようにするためです。
	// privateとするのは、発行したてのCSRFトークンのSet-Cookieを伴いうるレスポンス
	// であり、共有キャッシュが次の訪問者へ渡してはならないためです。
	w.Header().Set("Cache-Control", "private, no-store")

	meta := viewmodel.DefaultPageMeta(ctx, rd.cfg)
	meta.Title = i18n.T(ctx, "error_not_found_title")
	meta.Description = i18n.T(ctx, "error_not_found_message")

	var body bytes.Buffer
	if err := layouts.Default(meta, errorpages.NotFound()).Render(ctx, &body); err != nil {
		slog.ErrorContext(ctx, "404ページのレンダリングに失敗", "error", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.ErrorContext(ctx, "404レスポンスの書き込みに失敗", "error", err)
	}
}

// Forbiddenは403ページを応答します。UseCaseが権限不足を理由に要求を拒んだとき
// にページのハンドラーが呼び、どの画面でも拒否が同じページで応答されるようにします。
// 各画面がそれぞれ自前のものを書かずに済みます。
//
// ページはwへ何かが届く前にバッファ上で組み立てます。理由はNotFoundに記したとおりで、
// 描画に失敗しても、途中までのページを載せた200ではなく平文の403で応答できるように
// するためです。
func (rd *Renderer) Forbidden(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 拒否は誰が尋ねているかについての応答であり、他の誰かへ渡してよい答えでは
	// ありません。再検証の方針ではなくno-storeとするのは、訪問者がロールを持てば同じURLが
	// ページ自身で応答するためです。保存された拒否はその手前に立ってしまいます。
	w.Header().Set("Cache-Control", "private, no-store")

	meta := viewmodel.DefaultPageMeta(ctx, rd.cfg)
	meta.Title = i18n.T(ctx, "error_forbidden_title")
	meta.Description = i18n.T(ctx, "error_forbidden_message")
	// 拒否は実在する画面のアドレスを伴うため、そうしなければクローラーが記録しうる
	// ページです。ここに見つける価値のあるものは無く、誰が拒まれても同じページです。
	meta.NoIndex = true

	var body bytes.Buffer
	if err := layouts.Default(meta, errorpages.Forbidden()).Render(ctx, &body); err != nil {
		slog.ErrorContext(ctx, "403ページのレンダリングに失敗", "error", err)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.ErrorContext(ctx, "403レスポンスの書き込みに失敗", "error", err)
	}
}

// Unpublishedは、管理者が見えない場所へ移したリソースのページを応答します。UseCaseが
// AppErrCodeResourceUnpublishedを報告したときにページのハンドラーが呼び、内容が取り除かれた
// アドレスがどこでも同じページで応答するようにします。各画面がそれぞれ自前のものを書かずに
// 済みます。
//
// ステータスは404です。訪問者もクローラーもそれに従います。コミュニティはここで何も示さなく
// なったためです。410としないのは、印を外せばそのアドレスがまた応答しうる一方、Goneは二度と
// 応答しないことを述べるためです。
//
// ページはwへ何かが届く前にバッファ上で組み立てます。理由はNotFoundに記したとおりで、
// 失敗したときの応答は平文の404です。クローラーが読むのはステータスであるため、描画の失敗が
// 取り除かれたスレッドを成功に変えてはなりません。
func (rd *Renderer) Unpublished(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 他のエラーページが宣言するのと同じ方針で、理由も同じです。印を外せばそのアドレスは
	// スレッド自身で応答し、保存された応答はその手前に立ってしまいます。
	w.Header().Set("Cache-Control", "private, no-store")

	meta := viewmodel.DefaultPageMeta(ctx, rd.cfg)
	meta.Title = i18n.T(ctx, "error_unpublished_title")
	meta.Description = i18n.T(ctx, "error_unpublished_message")
	// このアドレスはコミュニティが応答していたものであり、そのスレッドは検索結果に
	// まだ残っているかもしれません。今ここに立つページは誰にとっても同じもので、見つける
	// 価値のあるものを持たないため、その代わりに記録されるべきページではありません。
	meta.NoIndex = true

	var body bytes.Buffer
	if err := layouts.Default(meta, errorpages.Unpublished()).Render(ctx, &body); err != nil {
		slog.ErrorContext(ctx, "非公開ページのレンダリングに失敗", "error", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.ErrorContext(ctx, "非公開ページのレスポンスの書き込みに失敗", "error", err)
	}
}
