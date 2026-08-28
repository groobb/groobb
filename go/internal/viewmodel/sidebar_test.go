package viewmodel_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// navigation returns the UseCase output the sidebar is built from: one community
// with one board, which is the smallest arrangement in which every data-backed
// part of the sidebar has something to render.
//
// [Ja] navigation はサイドバーの組み立て元となる UseCase の出力を返します。1 つの
// コミュニティと 1 つの掲示板であり、サイドバーのデータ由来の部分がすべて描くものを
// 持つ最小の構成です。
func navigation() *usecase.GetCommunityNavigationOutput {
	return &usecase.GetCommunityNavigationOutput{
		Community: &model.Community{ID: 1, Name: "ジャズ喫茶"},
		Boards:    []*model.Board{{ID: 1, Slug: "jazz", Name: "ジャズ・ファンク"}},
	}
}

// TestNewSidebar_AnonymousVisitor verifies that a nil user leaves the account
// controls out while the community and its boards are still carried, and that
// the page to come back to after signing in is. The public pages of the
// community are readable while signed out, so the sidebar is built for a visitor
// who has no account controls to show, and the CSRF token of the sign-out form
// must not be carried into a sidebar that renders no such form.
//
// [Ja] TestNewSidebar_AnonymousVisitor は、user が nil のときアカウント操作が外れる
// 一方で、コミュニティとその掲示板、そしてサインイン後に戻ってくる先のページが運ばれる
// ことを検証します。コミュニティの公開ページはサインアウト状態でも読めるため、サイド
// バーは表示すべきアカウント操作を持たない訪問者に対しても組み立てられます。そして
// サインアウトフォームを描かないサイドバーへ、そのフォームの CSRF トークンを運んでは
// なりません。
func TestNewSidebar_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	sidebar := viewmodel.NewSidebar(navigation(), nil, "csrf-token", "/b/jazz")

	if sidebar.SignedIn {
		t.Error("sidebar.SignedIn = true, want false")
	}
	if sidebar.Atname != "" {
		t.Errorf("sidebar.Atname = %q, want %q", sidebar.Atname, "")
	}
	if sidebar.CSRFToken != "" {
		t.Errorf("sidebar.CSRFToken = %q, want %q", sidebar.CSRFToken, "")
	}
	if sidebar.ReturnTo != "/b/jazz" {
		t.Errorf("sidebar.ReturnTo = %q, want %q", sidebar.ReturnTo, "/b/jazz")
	}
	if sidebar.CommunityName != "ジャズ喫茶" {
		t.Errorf("sidebar.CommunityName = %q, want %q", sidebar.CommunityName, "ジャズ喫茶")
	}
	if len(sidebar.Boards) != 1 {
		t.Fatalf("sidebar.Boards = %+v, want one board", sidebar.Boards)
	}
	if got := sidebar.Boards[0].Slug; got != "jazz" {
		t.Errorf("sidebar.Boards[0].Slug = %q, want %q", got, "jazz")
	}
}

// TestNewSidebar_SignedInVisitor verifies that a signed-in user brings the
// account controls with them, carrying the atname shown above them and the CSRF
// token the sign-out form submits, and that the destination of a sign-in link
// they are never shown is left behind.
//
// [Ja] TestNewSidebar_SignedInVisitor は、サインイン済みユーザーがアカウント操作を
// 伴うこと、すなわちその上に表示する atname と、サインアウトフォームが送信する CSRF
// トークンが運ばれること、そして彼らには決して描画されないサインインのリンクの遷移先が
// 置いていかれることを検証します。
func TestNewSidebar_SignedInVisitor(t *testing.T) {
	t.Parallel()

	sidebar := viewmodel.NewSidebar(navigation(), &model.User{Atname: "alice"}, "csrf-token", "/b/jazz")

	if !sidebar.SignedIn {
		t.Error("sidebar.SignedIn = false, want true")
	}
	if sidebar.Atname != "alice" {
		t.Errorf("sidebar.Atname = %q, want %q", sidebar.Atname, "alice")
	}
	if sidebar.CSRFToken != "csrf-token" {
		t.Errorf("sidebar.CSRFToken = %q, want %q", sidebar.CSRFToken, "csrf-token")
	}
	if sidebar.ReturnTo != "" {
		t.Errorf("sidebar.ReturnTo = %q, want %q", sidebar.ReturnTo, "")
	}
}

// TestNewSidebar_UnsetInstance verifies that an instance without a community row
// leaves the name empty rather than failing, since that is the state a freshly
// migrated database is in and the board navigation still has to be built.
//
// [Ja] TestNewSidebar_UnsetInstance は、コミュニティの行を持たないインスタンスでも
// 失敗せずに名前が空のままになることを検証します。それはマイグレーション直後の
// データベースが置かれている状態であり、板のナビゲーションはそれでも組み立てられなければ
// ならないためです。
func TestNewSidebar_UnsetInstance(t *testing.T) {
	t.Parallel()

	nav := navigation()
	nav.Community = nil

	sidebar := viewmodel.NewSidebar(nav, &model.User{Atname: "alice"}, "csrf-token", "")

	if sidebar.CommunityName != "" {
		t.Errorf("sidebar.CommunityName = %q, want %q", sidebar.CommunityName, "")
	}
	if len(sidebar.Boards) != 1 {
		t.Errorf("len(sidebar.Boards) = %d, want 1", len(sidebar.Boards))
	}
}
