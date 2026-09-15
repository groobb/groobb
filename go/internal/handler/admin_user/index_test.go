package admin_user_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/admin_user"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newAdminUserHandlerはテスト用データベースのリポジトリでadmin_user Handlerを
// 組み立てます。ハンドラーテストが、実際に保存されたアカウントとロールに対して一覧と
// その権限の判定を動かせるようにするためです。
func newAdminUserHandler(t *testing.T, db *database.DB) *admin_user.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	return admin_user.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		usecase.NewGetAdminUsersUsecase(repository.NewRoleRepository(db), repository.NewUserRepository(db)),
	)
}

// getAdminUsersは、与えられたクエリ文字列と、(RequireAuthが置くように) contextに
// 指定されたユーザーを載せたGET /admin/usersリクエストを組み立て、ハンドラーで応答します。
func getAdminUsers(t *testing.T, db *database.DB, userID model.UserID, locale model.Locale, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()

	path := templates.AdminUsersPath().String()
	if rawQuery != "" {
		path += "?" + rawQuery
	}

	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = templates.SetCurrentPath(ctx, path)
	ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "alice"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	newAdminUserHandler(t, db).Index(rec, req)
	return rec
}

// buildAdministratorは組み込みのadminロールを持つアカウントを作成します。
// 本ファイルのどの一覧も、この人によって読まれます。
func buildAdministrator(t *testing.T, db *database.DB, atname string) model.UserID {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
	return userID
}

// TestIndexは、管理者がGET /admin/usersを開くとHTTP 200と一覧で応答されることを
// 検証します。すなわち、キャプションが内容を名指し、見出しセルにスコープの付いた表、
// アカウントごとの行とそれが持つロールの名前、検索フィールド、そしてnoindexのrobots
// メタが、サポートする各ロケールで描かれることです。
func TestIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		locale                model.Locale
		wantHeading           string
		wantSearchLabel       string
		wantAdminRole         string
		wantGrantButton       string
		wantRevokeButton      string
		wantRevokeConfirm     string
		wantRevokeSelfConfirm string
	}{
		{
			name:                  "日本語",
			locale:                model.LocaleJa,
			wantHeading:           "利用者の一覧",
			wantSearchLabel:       "atnameで絞り込む",
			wantAdminRole:         "管理者",
			wantGrantButton:       "管理者にする",
			wantRevokeButton:      "管理者から外す",
			wantRevokeConfirm:     "@otheradminから管理者ロールを外しますか？",
			wantRevokeSelfConfirm: "自分から管理者ロールを外すと、この管理画面を開けなくなります。外しますか？",
		},
		{
			name:                  "英語",
			locale:                model.LocaleEn,
			wantHeading:           "User list",
			wantSearchLabel:       "Filter by atname",
			wantAdminRole:         "Administrator",
			wantGrantButton:       "Make administrator",
			wantRevokeButton:      "Remove administrator",
			wantRevokeConfirm:     "Remove the administrator role from @otheradmin?",
			wantRevokeSelfConfirm: "Removing your own administrator role will close these admin screens to you. Remove it?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			adminID := buildAdministrator(t, db, "adminuser")
			buildAdministrator(t, db, "otheradmin")
			testutil.NewUserBuilder(t, db).WithAtname("plainuser").Build()

			rec := getAdminUsers(t, db, adminID, tt.locale, "")

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantSearchLabel,
				tt.wantAdminRole,
				tt.wantGrantButton,
				tt.wantRevokeButton,
				`data-confirm="` + tt.wantRevokeConfirm + `"`,
				`data-confirm="` + tt.wantRevokeSelfConfirm + `"`,
				"@adminuser",
				"@otheradmin",
				"@plainuser",
				`<caption id="admin-user-index-table-caption"`,
				`aria-labelledby="admin-user-index-table-caption"`,
				`<th scope="col">`,
				`<th scope="row"`,
				`name="q"`,
				`<meta name="robots" content="noindex"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestIndex_Titleは、文書タイトルが絞り込みと2ページ目以降のページ番号を、個別にも
// 同時にも識別することを、サポートする各ロケールで検証します。絞り込みの無い1ページ目は
// 短い基本タイトルのままです。
func TestIndex_Title(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    model.Locale
		rawQuery  string
		wantTitle string
	}{
		{name: "日本語で絞り込みもページ送りもしない", locale: model.LocaleJa, wantTitle: "利用者の一覧"},
		{name: "日本語で絞り込む", locale: model.LocaleJa, rawQuery: "q=admin", wantTitle: "「admin」で絞り込んだ利用者の一覧"},
		{name: "日本語でページ送りする", locale: model.LocaleJa, rawQuery: "page=2", wantTitle: "利用者の一覧 (2ページ目)"},
		{name: "日本語で絞り込んでページ送りする", locale: model.LocaleJa, rawQuery: "page=2&q=admin", wantTitle: "「admin」で絞り込んだ利用者の一覧 (2ページ目)"},
		{name: "英語で絞り込みもページ送りもしない", locale: model.LocaleEn, wantTitle: "User list"},
		{name: "英語で絞り込む", locale: model.LocaleEn, rawQuery: "q=admin", wantTitle: "User list filtered by \"admin\""},
		{name: "英語でページ送りする", locale: model.LocaleEn, rawQuery: "page=2", wantTitle: "User list (Page 2)"},
		{name: "英語で絞り込んでページ送りする", locale: model.LocaleEn, rawQuery: "page=2&q=admin", wantTitle: "User list filtered by \"admin\" (Page 2)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			adminID := buildAdministrator(t, db, "adminuser")

			rec := getAdminUsers(t, db, adminID, tt.locale, tt.rawQuery)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			want := "<title>" + html.EscapeString(tt.wantTitle) + "</title>"
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("レスポンスボディに %q が含まれていない", want)
			}
		})
	}
}

// TestIndex_RoleButtonsは、各行がその行に当てはまるロールの操作を1つだけ差し出す
// こと (ロールを持つアカウントからは外す操作、持たないアカウントには与える操作)、そして
// そのそれぞれが、その操作のアドレスへ送信し、送信の検証に使われるCSRFトークンを運ぶ
// フォームであることを検証します。各ボタンが自身の行によって名付けられていることも
// 確かめます。単独で読まれたボタンが、どのアカウントに作用するのかを述べるためです。
func TestIndex_RoleButtons(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	adminID := buildAdministrator(t, db, "adminuser")
	plainID := testutil.NewUserBuilder(t, db).WithAtname("plainuser").Build()

	body := getAdminUsers(t, db, adminID, model.LocaleJa, "").Body.String()

	wants := []string{
		fmt.Sprintf(`id="admin-user-role-button-%d"`, adminID),
		fmt.Sprintf(`aria-labelledby="admin-user-role-button-%d admin-user-atname-%d"`, adminID, adminID),
		fmt.Sprintf(`id="admin-user-atname-%d"`, plainID),
		fmt.Sprintf(`aria-labelledby="admin-user-role-button-%d admin-user-atname-%d"`, plainID, plainID),
		"管理者から外す",
		"管理者にする",
		fmt.Sprintf(`action="/admin/users/%d/roles/admin"`, adminID),
		fmt.Sprintf(`action="/admin/users/%d/roles"`, plainID),
		`name="_method" value="DELETE"`,
		`name="csrf_token"`,
		`name="role_name" value="admin"`,
		`onsubmit="if (!confirm(this.dataset.confirm)) { event.preventDefault(); return false; }"`,
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	if strings.Contains(body, "disabled") {
		t.Error("ロールのボタンが無効のままになっている")
	}
}

// TestIndex_RevokeConfirmationは、剥奪が尋ねる問いが、それが誰の行であるかに
// よって変わることを検証します。他の管理者の行は、作用する相手のアカウントを名指します。
// ボタンの言葉は列に沿って繰り返され、誰に対してかを述べるのは行だけであるためです。
// 訪問者自身の行は、代わりに管理者を降りることの代償を述べます。そこで外れるのは読んで
// いる一覧そのものであり、そこにあるatnameは既に自分自身のものであるためです。
//
// 2つはページ全体からではなく、それぞれのフォームのタグから読み取ります。誤った行に
// 書かれた確認が、正しいものがどこか他所に書かれていることで見過ごされないようにするため
// です。
func TestIndex_RevokeConfirmation(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	adminID := buildAdministrator(t, db, "adminuser")
	otherID := buildAdministrator(t, db, "otheradmin")

	body := getAdminUsers(t, db, adminID, model.LocaleJa, "").Body.String()

	selfForm := testutil.OpeningTag(t, body, fmt.Sprintf(`action="/admin/users/%d/roles/admin"`, adminID))
	if want := `data-confirm="自分から管理者ロールを外すと、この管理画面を開けなくなります。外しますか？"`; !strings.Contains(selfForm, want) {
		t.Errorf("自分自身の行のフォーム = %s、%s を含むことを期待", selfForm, want)
	}

	otherForm := testutil.OpeningTag(t, body, fmt.Sprintf(`action="/admin/users/%d/roles/admin"`, otherID))
	if want := `data-confirm="@otheradminから管理者ロールを外しますか？"`; !strings.Contains(otherForm, want) {
		t.Errorf("他の管理者の行のフォーム = %s、%s を含むことを期待", otherForm, want)
	}
}

// TestIndex_Searchは、検索が一覧をその文字列で始まるatnameに絞り込むこと、
// フィールドが何で絞り込まれたのかを述べること、そして誰にも一致しない検索が、アカウントを
// 持たないコミュニティとしてではなく検索として、その旨を述べることを検証します。
func TestIndex_Search(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	adminID := buildAdministrator(t, db, "adminuser")
	testutil.NewUserBuilder(t, db).WithAtname("plainuser").Build()

	matched := getAdminUsers(t, db, adminID, model.LocaleJa, "q=plain").Body.String()
	if !strings.Contains(matched, "@plainuser") {
		t.Error("絞り込みに一致する利用者が一覧に描画されていない")
	}
	if strings.Contains(matched, "@adminuser") {
		t.Error("絞り込みに一致しない利用者が一覧に描画されている")
	}
	if !strings.Contains(matched, `value="plain"`) {
		t.Error("絞り込みの文字列が検索フィールドに書き戻されていない")
	}

	unmatched := getAdminUsers(t, db, adminID, model.LocaleJa, "q=nobody").Body.String()
	if !strings.Contains(unmatched, "「nobody」で始まるatnameの利用者はいません。") {
		t.Error("一致しない絞り込みの空状態の文言が描画されていない")
	}
	if strings.Contains(unmatched, "利用者がいません。") {
		t.Error("絞り込みに一致しない一覧が、利用者のいない一覧として描画されている")
	}
}

// TestIndex_Pagingは、1ページに収まらない一覧が次のページへの道とそこから戻る道を
// 差し出すこと、そして両方のリンクが一覧の絞り込みを保つことを検証します。検索したまま
// ページを送っても、全アカウントから始め直しにならないようにするためです。
func TestIndex_Paging(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	adminID := buildAdministrator(t, db, "u0admin")
	for i := range model.AdminUsersPerPage {
		testutil.NewUserBuilder(t, db).WithAtname(fmt.Sprintf("u%dpaged", i)).Build()
	}

	first := getAdminUsers(t, db, adminID, model.LocaleJa, "q=u").Body.String()
	if !strings.Contains(first, `href="/admin/users?page=2&amp;q=u"`) {
		t.Error("1ページ目に、絞り込みを保った次のページへのリンクが描画されていない")
	}
	if strings.Contains(first, "前のページ") {
		t.Error("1ページ目に前のページへのリンクが描画されている")
	}
	if !strings.Contains(first, "全2ページ中1ページ目") {
		t.Error("1ページ目にページの位置が描画されていない")
	}

	second := getAdminUsers(t, db, adminID, model.LocaleJa, "page=2&q=u").Body.String()
	if !strings.Contains(second, `href="/admin/users?q=u"`) {
		t.Error("2ページ目に、絞り込みを保った前のページへのリンクが描画されていない")
	}
	if strings.Contains(second, "次のページ") {
		t.Error("最後のページに次のページへのリンクが描画されている")
	}
	if !strings.Contains(second, "@u0admin") {
		t.Error("2ページ目に、最初のページに収まらなかった利用者が描画されていない")
	}
}

// TestIndex_PageNotFoundは、整数として読めない値、または最初のページより前の整数を
// 運ぶアドレスが、404ページで応答されることを検証します。最後のページより後ろの整数は、
// その下で別に検証します。
func TestIndex_PageNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawQuery string
	}{
		{name: "整数でないページ番号", rawQuery: "page=abc"},
		{name: "最初のページより前のページ番号", rawQuery: "page=0"},
		{name: "負のページ番号", rawQuery: "page=-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			adminID := buildAdministrator(t, db, "adminuser")

			rec := getAdminUsers(t, db, adminID, model.LocaleJa, tt.rawQuery)

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}
			if strings.Contains(rec.Body.String(), "@adminuser") {
				t.Error("存在しないページの応答に一覧の中身が描画されている")
			}
		})
	}
}

// TestIndex_PastLastPageは、最後のページより後ろのページ番号が、空で描かれ戻る道を
// 差し出す一覧自身で応答されることを、絞り込みの有無の両方で検証します。一覧全体に
// アカウントや一致するものが無いとは表示しません。番号がどこまで届くかはその時点の
// アカウントの数で決まるため、これはどのページも名指さないアドレスではなく、最後を
// 通り過ぎてページを送った一覧です。
//
// 戻る道は、読まれている番号の1つ下ではなく一覧が振る最後のページへ導きます。その間の
// どの番号も同じ理由で終端の外にあるためです。他のページのどこに位置するかは述べません。
// 一覧が番号を振っていないページに、その位置は無いためです。
func TestIndex_PastLastPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		rawQuery         string
		wantPreviousLink string
		wantNotEmptyText string
	}{
		{
			name:             "絞り込みなし",
			rawQuery:         "page=3",
			wantPreviousLink: `href="/admin/users"`,
			wantNotEmptyText: "利用者がいません。",
		},
		{
			name:             "絞り込みあり",
			rawQuery:         "page=3&q=admin",
			wantPreviousLink: `href="/admin/users?q=admin"`,
			wantNotEmptyText: "「admin」で始まるatnameの利用者はいません。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			adminID := buildAdministrator(t, db, "adminuser")

			rec := getAdminUsers(t, db, adminID, model.LocaleJa, tt.rawQuery)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if strings.Contains(body, "@adminuser") {
				t.Error("最後のページより後ろのページに利用者が描画されている")
			}
			if !strings.Contains(body, "このページに表示する利用者はいません。前のページに戻ってください。") {
				t.Error("最後のページより後ろであることを示す空状態の文言が描画されていない")
			}
			if strings.Contains(body, tt.wantNotEmptyText) {
				t.Errorf("最後のページより後ろの一覧に、一覧全体が空であるかのような文言 %q が描画されている", tt.wantNotEmptyText)
			}
			if !strings.Contains(body, tt.wantPreviousLink) {
				t.Error("最後のページより後ろのページに、最後のページへ戻るリンクが描画されていない")
			}
			if strings.Contains(body, "全1ページ中3ページ目") {
				t.Error("最後のページより後ろのページに、一覧が振っていない位置が描画されている")
			}
		})
	}
}

// TestIndex_UnnumberedPageOfEmptyListingは、何も一致しなかった一覧の2ページ目
// 以降が、ページ送りを一切差し出さないことを検証します。一覧はどのページにも番号を
// 振っておらず、戻る先が無いためです。戻るリンクを描けば、そこもアカウントの無いページに
// なります。
func TestIndex_UnnumberedPageOfEmptyListing(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	adminID := buildAdministrator(t, db, "adminuser")

	rec := getAdminUsers(t, db, adminID, model.LocaleJa, "page=2&q=nobody")

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "「nobody」で始まるatnameの利用者はいません。") {
		t.Error("一致しない絞り込みの空状態の文言が描画されていない")
	}
	if strings.Contains(body, "ページ送り") {
		t.Error("どのページも持たない一覧にページ送りが描画されている")
	}
	if strings.Contains(body, "前のページ") {
		t.Error("戻る先を持たない一覧に前のページへのリンクが描画されている")
	}
}

// TestIndex_Forbiddenは、ロールを1つも持たないサインイン済みの訪問者が、一覧では
// なく403ページで応答されることを検証します。サインインしていること自体は、コミュニティ
// のアカウント名を読んでよいという意味ではありません。
func TestIndex_Forbidden(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).WithAtname("plainuser").Build()

	rec := getAdminUsers(t, db, userID, model.LocaleJa, "")

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}
	if strings.Contains(rec.Body.String(), "@plainuser") {
		t.Error("権限の無い訪問者へ利用者の一覧が描画されている")
	}
}

// TestIndex_WithoutUserは、RequireAuthが保証するユーザー無しでハンドラーへ到達した
// 場合に、panicやコミュニティのアカウントを誰でもない人へ並べることではなく、Internal
// Server Errorで応答することを検証します。
func TestIndex_WithoutUser(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	req := httptest.NewRequest(http.MethodGet, templates.AdminUsersPath().String(), nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	rec := httptest.NewRecorder()

	newAdminUserHandler(t, db).Index(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "Internal Server Error\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Internal Server Error\n")
	}
}
