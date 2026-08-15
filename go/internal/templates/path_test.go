package templates_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/templates"
)

// TestStaticPaths verifies that the no-argument path helpers return the exact
// route strings registered in cmd/server/main.go, so the two never drift apart.
//
// [Ja] TestStaticPaths は引数なしのパスヘルパーが cmd/server/main.go で登録された
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
		{name: "CommunityNewPath", got: templates.CommunityNewPath(), want: "/communities/new"},
		{name: "CommunityListPath", got: templates.CommunityListPath(), want: "/communities"},
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

// TestCommunityPath verifies that a community's page is addressed by the short
// /c/{identifier} path rather than under the /communities collection namespace.
//
// [Ja] TestCommunityPath は、コミュニティの画面が /communities のコレクション名前空間の
// 下ではなく短縮パス /c/{identifier} で指されることを検証します。
func TestCommunityPath(t *testing.T) {
	t.Parallel()

	if got := templates.CommunityPath("groobb"); got != "/c/groobb" {
		t.Errorf("CommunityPath(%q) = %q, want %q", "groobb", got, "/c/groobb")
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
			returnTo: "/c/groobb",
			want:     "/sign_in?return_to=%2Fc%2Fgroobb",
		},
		{
			name:     "クエリを含む遷移先をエンコードする",
			path:     templates.SignInTwoFactorRecoveryNewPath(),
			returnTo: "/c/groobb?tab=posts",
			want:     "/sign_in/two_factor/recovery/new?return_to=%2Fc%2Fgroobb%3Ftab%3Dposts",
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

	if got := templates.AfterSignInPath("/c/groobb"); got != "/c/groobb" {
		t.Errorf("AfterSignInPath(%q) = %q, want %q", "/c/groobb", got, "/c/groobb")
	}
	if got := templates.AfterSignInPath(""); got != templates.HomePath() {
		t.Errorf("AfterSignInPath(%q) = %q, want %q", "", got, templates.HomePath())
	}
}
