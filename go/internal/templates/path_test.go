package templates_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/templates"
)

// TestStaticPaths verifies that the no-argument path helpers return the exact
// route strings registered in cmd/groobb/serve.go, so the two never drift apart.
//
// [Ja] TestStaticPaths は引数なしのパスヘルパーが cmd/groobb/serve.go で登録された
// ルート文字列と完全に一致することを検証し、両者が乖離しないようにします。
func TestStaticPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  templates.Path
		want templates.Path
	}{
		{name: "RootPath", got: templates.RootPath(), want: "/"},
		{name: "SignUpPath", got: templates.SignUpPath(), want: "/sign_up"},
		{name: "SignInPath", got: templates.SignInPath(), want: "/sign_in"},
		{name: "SignInTwoFactorNewPath", got: templates.SignInTwoFactorNewPath(), want: "/sign_in/two_factor/new"},
		{name: "SignInTwoFactorPath", got: templates.SignInTwoFactorPath(), want: "/sign_in/two_factor"},
		{name: "SignInTwoFactorRecoveryNewPath", got: templates.SignInTwoFactorRecoveryNewPath(), want: "/sign_in/two_factor/recovery/new"},
		{name: "SignInTwoFactorRecoveryPath", got: templates.SignInTwoFactorRecoveryPath(), want: "/sign_in/two_factor/recovery"},
		{name: "HomePath", got: templates.HomePath(), want: "/home"},
		{name: "UserSessionPath", got: templates.UserSessionPath(), want: "/user_session"},
		{name: "SettingsPath", got: templates.SettingsPath(), want: "/settings"},
		{name: "SettingsEmailEditPath", got: templates.SettingsEmailEditPath(), want: "/settings/email/edit"},
		{name: "SettingsEmailPath", got: templates.SettingsEmailPath(), want: "/settings/email"},
		{name: "SettingsEmailConfirmationNewPath", got: templates.SettingsEmailConfirmationNewPath(), want: "/settings/email/confirmation/new"},
		{name: "SettingsEmailConfirmationPath", got: templates.SettingsEmailConfirmationPath(), want: "/settings/email/confirmation"},
		{name: "SettingsTwoFactorAuthNewPath", got: templates.SettingsTwoFactorAuthNewPath(), want: "/settings/two_factor_auth/new"},
		{name: "SettingsTwoFactorAuthPath", got: templates.SettingsTwoFactorAuthPath(), want: "/settings/two_factor_auth"},
		{name: "SettingsWithdrawalNewPath", got: templates.SettingsWithdrawalNewPath(), want: "/settings/withdrawal/new"},
		{name: "SettingsWithdrawalPath", got: templates.SettingsWithdrawalPath(), want: "/settings/withdrawal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestPath_AbsoluteURL verifies that a path is named under the instance's
// public base URL, and that an instance which has not been told its own address
// yields nothing rather than a host-relative one. The callers publish these to
// machines that read them away from the page they were written on, where a path
// by itself names no host.
//
// [Ja] TestPath_AbsoluteURL は、パスがインスタンスの公開ベース URL の下で名指される
// こと、そして自身のアドレスを教えられていないインスタンスでは、ホスト相対の URL では
// なく何も返らないことを検証します。呼び出し側はこれらを、それが書かれたページから離れて
// 読む機械に向けて公開します。そこではパスだけではどのホストのことかが定まりません。
func TestPath_AbsoluteURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    templates.Path
		baseURL string
		want    string
	}{
		{
			name:    "ベース URL の下の絶対 URL になる",
			path:    templates.BoardPath("jazz"),
			baseURL: "https://groobb.example.com",
			want:    "https://groobb.example.com/b/jazz",
		},
		{
			name:    "ベース URL が空なら空になる",
			path:    templates.BoardPath("jazz"),
			baseURL: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.path.AbsoluteURL(tt.baseURL); got != tt.want {
				t.Errorf("AbsoluteURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

// TestPath_WithReturnTo verifies that a destination is attached as an encoded
// return_to query parameter, and that an empty one leaves the path untouched so
// a flow carrying no destination links to the bare path.
//
// [Ja] TestPath_WithReturnTo は遷移先がエンコードされた return_to クエリパラメータとして
// 付くこと、そして空のときはパスがそのままになり、遷移先を持たないフローが素のパスへ
// リンクすることを検証します。
func TestPath_WithReturnTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     templates.Path
		returnTo string
		want     templates.Path
	}{
		{
			name:     "遷移先を付ける",
			path:     templates.SignInPath(),
			returnTo: "/settings",
			want:     "/sign_in?return_to=%2Fsettings",
		},
		{
			name:     "クエリを含む遷移先をエンコードする",
			path:     templates.SignInTwoFactorRecoveryNewPath(),
			returnTo: "/settings?from=home",
			want:     "/sign_in/two_factor/recovery/new?return_to=%2Fsettings%3Ffrom%3Dhome",
		},
		{
			name:     "空の遷移先ではパスを変えない",
			path:     templates.SignInPath(),
			returnTo: "",
			want:     "/sign_in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.path.WithReturnTo(tt.returnTo); got != tt.want {
				t.Errorf("WithReturnTo(%q) = %q, want %q", tt.returnTo, got, tt.want)
			}
		})
	}
}

// TestAfterSignInPath verifies that a session-issuing route lands the visitor on
// the destination the flow carried, and on the home page when it carried none.
// Home rather than the top page keeps sign-in from redirecting twice, since the
// top page would only send a signed-in visitor on to home.
//
// [Ja] TestAfterSignInPath は、セッションを発行するルートが、フローの運んできた遷移先へ、
// 運んでこなかったときはホームへ訪問者を着地させることを検証します。トップページではなく
// ホームであることで、サインインが 2 段リダイレクトにならない (トップページはサインイン済みの
// 訪問者をホームへ送るだけであるため)。
func TestAfterSignInPath(t *testing.T) {
	t.Parallel()

	if got := templates.AfterSignInPath("/settings"); got != "/settings" {
		t.Errorf("AfterSignInPath(%q) = %q, want %q", "/settings", got, "/settings")
	}
	if got := templates.AfterSignInPath(""); got != templates.HomePath() {
		t.Errorf("AfterSignInPath(%q) = %q, want %q", "", got, templates.HomePath())
	}
}

// TestCategoryPath verifies that a category's slug is placed under the /c prefix
// the category route is registered on, so a sidebar link and the route stay in
// step.
//
// [Ja] TestCategoryPath はカテゴリーの slug が、カテゴリーのルートが登録されている
// /c の接頭辞の下に置かれることを検証します。サイドバーのリンクとルートが乖離しない
// ためです。
func TestCategoryPath(t *testing.T) {
	t.Parallel()

	if got, want := templates.CategoryPath("music"), templates.Path("/c/music"); got != want {
		t.Errorf("CategoryPath(%q) = %q, want %q", "music", got, want)
	}
}

// TestBoardPath verifies that a board's slug is placed under the /b prefix the
// board route is registered on, so a sidebar link and the route stay in step.
//
// [Ja] TestBoardPath は掲示板の slug が、掲示板のルートが登録されている /b の接頭辞の
// 下に置かれることを検証します。サイドバーのリンクとルートが乖離しないためです。
func TestBoardPath(t *testing.T) {
	t.Parallel()

	if got, want := templates.BoardPath("jazz"), templates.Path("/b/jazz"); got != want {
		t.Errorf("BoardPath(%q) = %q, want %q", "jazz", got, want)
	}
}

// TestPostElementID verifies that a reply number becomes the id the post is
// rendered with, since that is what an anchor ending in #p12 has to find on the
// page.
//
// [Ja] TestPostElementID はレス番号が、その投稿が描画される際の id になることを検証
// します。#p12 で終わるアンカーがページ上で見つけねばならないものがこれであるためです。
func TestPostElementID(t *testing.T) {
	t.Parallel()

	if got, want := templates.PostElementID(12), "p12"; got != want {
		t.Errorf("PostElementID(%d) = %q, want %q", 12, got, want)
	}
}

// TestPostAnchor verifies that a link to a post is a same-document reference to
// that post's id, so a >>N in a body leads to the element PostElementID names.
//
// [Ja] TestPostAnchor は投稿へのリンクが、その投稿の id への同一文書内の参照になること
// を検証します。本文の >>N が PostElementID の名指す要素へ繋がるようにするためです。
func TestPostAnchor(t *testing.T) {
	t.Parallel()

	if got, want := templates.PostAnchor(12), templates.Path("#p12"); got != want {
		t.Errorf("PostAnchor(%d) = %q, want %q", 12, got, want)
	}
}
