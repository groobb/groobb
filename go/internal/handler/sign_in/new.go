package sign_in

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	signinpage "github.com/groobb/groobb/go/internal/templates/pages/sign_in"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /sign_in - renders the sign-in form with a fresh CSRF token. A visitor
// sent here from a guarded route arrives with return_to naming where they were
// headed; it is validated before it goes into the form, so only a destination we
// would redirect to is echoed back into the page.
//
// [Ja] New GET /sign_in - 新しい CSRF トークン付きでサインインフォームを描画します。
// 保護されたルートからここへ送られた訪問者は、向かっていた先を表す return_to を伴って
// 到達します。フォームに載せる前に検証するため、ページにエコーバックされるのは実際に
// リダイレクトしうる遷移先だけです。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	h.renderNew(w, r, http.StatusOK, query.Has(templates.ReturnToParam), signinpage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		ReturnTo:  middleware.SanitizeReturnTo(query.Get(templates.ReturnToParam)),
	})
}

// renderNew renders the sign-in form with the given status, indexing policy, and
// data. It is shared by New (200) and Create's re-render after a validation error
// (422). The status is written before rendering, so callers pass the final status
// here rather than setting it separately. noIndex says whether the URL being
// answered is a return_to variant of the form; only the GET render can be one, so
// Create's re-render (answered at the bare /sign_in) always passes false.
//
// [Ja] renderNew は指定したステータス・インデックス方針・データでサインインフォームを
// 描画します。New (200) と、Create のバリデーションエラー後の再描画 (422) で共有します。
// ステータスは描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを
// 渡します。noIndex は、応答している URL がフォームの return_to バリアントかどうかを表します。
// バリアントになりうるのは GET の描画だけであるため、素の /sign_in で応答する Create の
// 再描画は常に false を渡します。
func (h *Handler) renderNew(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	noIndex bool,
	data signinpage.NewPageData,
) {
	ctx := r.Context()

	// The Turnstile site key is the same for every render (it comes from config,
	// not the request), so set it here once rather than at each call site. An empty
	// key (the disabled dev / test setup) makes the widget render nothing.
	//
	// [Ja] Turnstile のサイトキーはどの描画でも同じ (リクエストではなく config 由来) なので、
	// 各呼び出し側ではなくここで一度だけ設定する。キーが空 (無効化された dev / test 構成) の
	// ときはウィジェットを何も描画しない。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "sign_in_new_title")
	meta.Description = i18n.T(ctx, "sign_in_new_description")

	// A return_to render is the same form under a per-destination URL, and every
	// guarded route can produce one, so those variants are kept out of search
	// results. The bare /sign_in — the form's one representative address — stays
	// indexable.
	//
	// [Ja] return_to 付きの描画は遷移先ごとの URL に同じフォームが出ているだけで、
	// 保護されたルートの数だけ生まれうるため、これらのバリアントは検索結果に出さない。
	// フォームの代表アドレスである素の /sign_in はインデックス対象のままとする。
	meta.NoIndex = noIndex

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, signinpage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "サインインページのレンダリングに失敗", "error", err)
	}
}
