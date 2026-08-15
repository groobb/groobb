package community

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	communitypage "github.com/groobb/groobb/go/internal/templates/pages/community"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /communities - creates the community from the submitted name and
// URL identifier, together with the administrator role the signed-in creator is
// given, and sends the creator to the new community with a success flash. It is
// registered behind RequireAuth, so the user from the context is non-nil. On a
// validation error the form is re-rendered with the messages (422). The CSRF
// check is enforced upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Create POST /communities - 送信された名前と URL 識別子から、サインイン済みの作成者へ
// 与える管理者ロールと併せてコミュニティを作成し、作成者を成功フラッシュ付きで新しい
// コミュニティへ送ります。RequireAuth の背後に登録されるため、context のユーザーは非 nil です。
// バリデーションエラー時はメッセージ付きでフォームを再描画します (422)。CSRF 検証は上流の
// CSRF ミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	name := r.FormValue("name")
	identifier := r.FormValue("identifier")

	output, err := h.createCommunityUC.Execute(ctx, usecase.CreateCommunityInput{
		UserID:     user.ID,
		Name:       name,
		Identifier: identifier,
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			// Echo both submitted values back so the user only corrects the field
			// that was rejected instead of filling the form in again.
			//
			// [Ja] 送信された値は両方エコーバックし、ユーザーがフォームを埋め直すのではなく
			// 弾かれたフィールドだけを直せばよいようにする。
			h.renderNew(w, r, http.StatusUnprocessableEntity, communitypage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Name:       name,
				Identifier: identifier,
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "コミュニティの作成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_community_created"))
	http.Redirect(w, r, templates.CommunityPath(output.Community.Identifier).String(), http.StatusSeeOther)
}
