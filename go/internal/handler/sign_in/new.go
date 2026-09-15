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

// New GET /sign_in - 新しいCSRFトークン付きでサインインフォームを描画します。
// 保護されたルートからここへ送られた訪問者は、向かっていた先を表すreturn_toを伴って
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

// renderNewは指定したステータス・インデックス方針・データでサインインフォームを
// 描画します。New (200) と、Createのバリデーションエラー後の再描画 (422) で共有します。
// ステータスは描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを
// 渡します。noIndexは、応答しているURLがフォームのreturn_toバリアントかどうかを表します。
// バリアントになりうるのはGETの描画だけであるため、素の /sign_inで応答するCreateの
// 再描画は常にfalseを渡します。
func (h *Handler) renderNew(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	noIndex bool,
	data signinpage.NewPageData,
) {
	ctx := r.Context()

	// Turnstileのサイトキーはどの描画でも同じ (リクエストではなくconfig由来) なので、
	// 各呼び出し側ではなくここで一度だけ設定する。キーが空 (無効化されたdev / test構成) の
	// ときはウィジェットを何も描画しない。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "sign_in_new_title")
	meta.Description = i18n.T(ctx, "sign_in_new_description")

	// return_to付きの描画は遷移先ごとのURLに同じフォームが出ているだけで、
	// 保護されたルートの数だけ生まれうるため、これらのバリアントは検索結果に出さない。
	// フォームの代表アドレスである素の /sign_inはインデックス対象のままとする。
	meta.NoIndex = noIndex

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, signinpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "サインインページのレンダリングに失敗", "error", err)
	}
}
