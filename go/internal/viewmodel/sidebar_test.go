package viewmodel_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// navigationはサイドバーの組み立て元となるUseCaseの出力を返します。1つの
// コミュニティと1つの掲示板であり、サイドバーのデータ由来の部分がすべて描くものを
// 持つ最小の構成です。
func navigation() *usecase.GetCommunityNavigationOutput {
	return &usecase.GetCommunityNavigationOutput{
		Community: &model.Community{ID: 1, Name: "ジャズ喫茶"},
		Boards:    []*model.Board{{ID: 1, Slug: "jazz", Name: "ジャズ・ファンク"}},
	}
}

// TestNewSidebar_AnonymousVisitorは、userがnilのときアカウント操作が外れる
// 一方で、コミュニティとその掲示板、そしてサインイン後に戻ってくる先のページが運ばれる
// ことを検証します。コミュニティの公開ページはサインアウト状態でも読めるため、サイド
// バーは表示すべきアカウント操作を持たない訪問者に対しても組み立てられます。そして
// サインアウトフォームを描かないサイドバーへ、そのフォームのCSRFトークンを運んでは
// なりません。
func TestNewSidebar_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	sidebar := viewmodel.NewSidebar(navigation(), nil, "csrf-token", "/b/jazz")

	if sidebar.SignedIn {
		t.Error("sidebar.SignedIn = true、期待値 = false")
	}
	if sidebar.Atname != "" {
		t.Errorf("sidebar.Atname = %q、期待値 = %q", sidebar.Atname, "")
	}
	if sidebar.CSRFToken != "" {
		t.Errorf("sidebar.CSRFToken = %q、期待値 = %q", sidebar.CSRFToken, "")
	}
	if sidebar.CanAccessAdmin {
		t.Error("sidebar.CanAccessAdmin = true、期待値 = false")
	}
	if sidebar.ReturnTo != "/b/jazz" {
		t.Errorf("sidebar.ReturnTo = %q、期待値 = %q", sidebar.ReturnTo, "/b/jazz")
	}
	if sidebar.CommunityName != "ジャズ喫茶" {
		t.Errorf("sidebar.CommunityName = %q、期待値 = %q", sidebar.CommunityName, "ジャズ喫茶")
	}
	if len(sidebar.Boards) != 1 {
		t.Fatalf("sidebar.Boards = %+v、期待値は掲示板1件", sidebar.Boards)
	}
	if got := sidebar.Boards[0].Slug; got != "jazz" {
		t.Errorf("sidebar.Boards[0].Slug = %q、期待値 = %q", got, "jazz")
	}
}

// TestNewSidebar_SignedInVisitorは、サインイン済みユーザーがアカウント操作を
// 伴うこと、すなわちその上に表示するatname、サインアウトフォームが送信するCSRF
// トークン、そして管理画面が開かれているかどうかが運ばれること、そして彼らには決して
// 描画されないサインインのリンクの遷移先が置いていかれることを検証します。
func TestNewSidebar_SignedInVisitor(t *testing.T) {
	t.Parallel()

	nav := navigation()
	nav.CanAccessAdmin = true

	sidebar := viewmodel.NewSidebar(nav, &model.User{Atname: "alice"}, "csrf-token", "/b/jazz")

	if !sidebar.SignedIn {
		t.Error("sidebar.SignedIn = false、期待値 = true")
	}
	if !sidebar.CanAccessAdmin {
		t.Error("sidebar.CanAccessAdmin = false、期待値 = true")
	}
	if sidebar.Atname != "alice" {
		t.Errorf("sidebar.Atname = %q、期待値 = %q", sidebar.Atname, "alice")
	}
	if sidebar.CSRFToken != "csrf-token" {
		t.Errorf("sidebar.CSRFToken = %q、期待値 = %q", sidebar.CSRFToken, "csrf-token")
	}
	if sidebar.ReturnTo != "" {
		t.Errorf("sidebar.ReturnTo = %q、期待値 = %q", sidebar.ReturnTo, "")
	}
}

// TestNewSidebar_UnsetInstanceは、コミュニティの行を持たないインスタンスでも
// 失敗せずに名前が空のままになることを検証します。それはマイグレーション直後の
// データベースが置かれている状態であり、板のナビゲーションはそれでも組み立てられなければ
// ならないためです。
func TestNewSidebar_UnsetInstance(t *testing.T) {
	t.Parallel()

	nav := navigation()
	nav.Community = nil

	sidebar := viewmodel.NewSidebar(nav, &model.User{Atname: "alice"}, "csrf-token", "")

	if sidebar.CommunityName != "" {
		t.Errorf("sidebar.CommunityName = %q、期待値 = %q", sidebar.CommunityName, "")
	}
	if len(sidebar.Boards) != 1 {
		t.Errorf("len(sidebar.Boards) = %d、期待値 = 1", len(sidebar.Boards))
	}
}
