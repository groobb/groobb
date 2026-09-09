package admin_user

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	adminuserpage "github.com/groobb/groobb/go/internal/templates/pages/admin_user"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Index GET /admin/users - renders one page of the community's accounts,
// narrowed to the atnames beginning with what the search carries. It is
// registered behind RequireAuth, which settles that someone is signed in;
// whether that someone may read the listing is settled by the UseCase, and a
// refusal is answered with the shared 403 page.
//
// A page number that is not a whole number, or is below the first page, names no
// page of the listing and is answered with the 404 page. A number past the last
// page is answered with the empty listing and a way back: the last page can
// change as accounts are added or removed after an address has been kept.
//
// The page is marked noindex for the reason the admin hub is: it is behind
// authentication and admitted to a few people, and what it holds is the
// community's account names.
//
// [Ja] Index GET /admin/users - コミュニティのアカウントの 1 ページを、検索が運ぶ文字列で
// 始まる atname に絞り込んで描画します。RequireAuth の背後に登録され、そこで誰かが
// サインインしていることが決まります。その誰かが一覧を読んでよいかどうかを決めるのは
// UseCase で、拒否には共通の 403 ページで応答します。
//
// 整数でないページ番号と、最初のページより前の番号は一覧のどのページも名指していないため、
// 404 ページで応答します。最後のページより後ろの番号には、空の一覧と戻る道で応答します。
// アドレスを保存した後にアカウントが増減し、最後のページが変わることがあるためです。
//
// noindex を付ける理由は管理ハブと同じです。認証の背後にあり、許されるのは数人であり、
// そしてここが持つのはコミュニティのアカウント名です。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		slog.ErrorContext(ctx, "利用者一覧にユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	query := r.URL.Query()
	page, ok := parsePage(query.Get(templates.PageParam))
	if !ok {
		slog.InfoContext(ctx, "ページ番号として読めない値で利用者一覧に到達", "page", query.Get(templates.PageParam))
		h.errorRenderer.NotFound(w, r)
		return
	}
	atnamePrefix := query.Get(templates.AdminUsersQueryParam)

	listing, err := h.getAdminUsersUC.Execute(ctx, usecase.GetAdminUsersInput{
		Actor:        usecase.UserActor(user.ID),
		AtnamePrefix: atnamePrefix,
		Page:         page,
	})
	if err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) {
			switch ae.Code {
			case model.AppErrCodeForbidden:
				slog.InfoContext(ctx, ae.LogString())
				h.errorRenderer.Forbidden(w, r)
				return
			case model.AppErrCodeResourceNotFound:
				slog.InfoContext(ctx, ae.LogString())
				h.errorRenderer.NotFound(w, r)
				return
			}
		}
		slog.ErrorContext(ctx, "利用者一覧の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = indexTitle(ctx, atnamePrefix, page)
	meta.NoIndex = true
	meta.SignedIn = true

	pageData := adminuserpage.IndexPageData{
		AtnamePrefix: atnamePrefix,
		TotalCount:   listing.TotalCount,
		Users:        indexUsers(ctx, listing.Users, user.ID),
		Pagination: adminuserpage.IndexPagination{
			Page:       page,
			TotalPages: totalPages(listing.TotalCount),
		},
		CSRFToken:     middleware.CSRFTokenFromContext(ctx),
		AdminRoleName: string(model.RoleNameAdmin),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, adminuserpage.Index(pageData)).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "利用者一覧のレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// indexTitle names the exact listing being read: its filter when one is
// present, and its page number after the first page. Keeping those states in the
// title lets browser history, tabs, and assistive technology distinguish URLs
// whose rows differ.
//
// [Ja] indexTitle は、読まれている一覧を正確に名付けます。絞り込みがあればその文字列を、
// 2 ページ目以降ならページ番号を含めます。行の異なる URL を、ブラウザの履歴やタブ、
// 支援技術がタイトルから見分けられるようにするためです。
func indexTitle(ctx context.Context, atnamePrefix string, page int) string {
	templateData := map[string]any{"Query": atnamePrefix, "Page": page}

	if atnamePrefix != "" && page > 1 {
		return i18n.T(ctx, "admin_user_index_title_filtered_paginated", templateData)
	}
	if atnamePrefix != "" {
		return i18n.T(ctx, "admin_user_index_title_filtered", templateData)
	}
	if page > 1 {
		return i18n.T(ctx, "admin_user_index_title_paginated", templateData)
	}

	return i18n.T(ctx, "admin_user_index_title")
}

// parsePage reads the page number the address carries, and reports whether it
// names a page at all. An address without the parameter is the first page, which
// is the page the listing is opened at and the one the search form submits to.
//
// A value that is not a whole number names no page. It is refused rather than
// read leniently as one: "3 apples" is not the third page of anything, and a
// listing that answered it with a page would be inventing what the address says.
//
// A whole number below the first page is left to the UseCase, which answers it
// as a page that does not exist. The lower boundary of the listing is then
// enforced in one place.
//
// [Ja] parsePage はアドレスが運ぶページ番号を読み、それがそもそもページを名指している
// かどうかを返します。パラメータを持たないアドレスは最初のページです。それが一覧を開く
// ページであり、検索フォームの送信先でもあります。
//
// 整数でない値はどのページも名指していません。寛容に読み替えるのではなく拒むのは、
// 「3 apples」が何かの 3 ページ目ではなく、それにページで応答する一覧は、アドレスが
// 述べていないことを作り出すことになるためです。
//
// 最初のページより前の整数は UseCase に委ね、存在しないページとして答えさせます。これに
// より、一覧の下限を適用する場所が 1 つだけになります。
func parsePage(value string) (int, bool) {
	if value == "" {
		return 1, true
	}

	page, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return page, true
}

// totalPages is how many pages the listing runs to, which is what the paging
// links compare the page being read against. A listing nothing matched runs to
// none, rather than to one empty page.
//
// [Ja] totalPages は一覧がどこまでのページを持つかであり、ページ送りのリンクが、読まれて
// いるページと比べる相手です。何も一致しなかった一覧は、空の 1 ページではなく、どの
// ページも持ちません。
func totalPages(totalCount int) int {
	return (totalCount + model.AdminUsersPerPage - 1) / model.AdminUsersPerPage
}

// indexUsers converts the accounts the UseCase read into the rows the page
// draws, naming each role as the page shows it and marking the row of the
// account doing the reading.
//
// [Ja] indexUsers は UseCase が読んだアカウントを、ページが描く行へ変換し、各ロールを
// ページが見せる形で名指し、読んでいるアカウント自身の行に印を付けます。
func indexUsers(ctx context.Context, users []usecase.AdminUser, actorID model.UserID) []adminuserpage.IndexUser {
	rows := make([]adminuserpage.IndexUser, len(users))
	for i, user := range users {
		roleNames := make([]string, len(user.Roles))
		for j, role := range user.Roles {
			roleNames[j] = roleDisplayName(ctx, role.Name)
		}

		rows[i] = adminuserpage.IndexUser{
			ID:         viewmodel.UserID(user.User.ID),
			Atname:     user.User.Atname,
			CreatedAt:  user.User.CreatedAt,
			RoleNames:  roleNames,
			HoldsAdmin: slices.ContainsFunc(user.Roles, func(role *model.Role) bool { return role.Name == model.RoleNameAdmin }),
			IsSelf:     user.User.ID == actorID,
		}
	}

	return rows
}

// roleDisplayName is what the listing calls a role. The role the instance ships
// with is read under its translated name, since it means the same thing in both
// UI languages and only one of them is what it is stored as. A role the
// community named itself is drawn as the community wrote it: nothing here knows
// a translation for a name this instance invented.
//
// [Ja] roleDisplayName は、一覧がロールを何と呼ぶかです。インスタンスに同梱される
// ロールは訳された名前で読まれます。それはどちらの UI 言語でも同じものを意味し、保存
// されているのはそのうちの一方に過ぎないためです。コミュニティが自ら名付けたロールは、
// コミュニティが書いたとおりに描きます。このインスタンスが考えた名前の訳語を知るものは
// ここに無いためです。
func roleDisplayName(ctx context.Context, name model.RoleName) string {
	if name == model.RoleNameAdmin {
		return i18n.T(ctx, "role_name_admin")
	}

	return string(name)
}
