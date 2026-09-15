package templates_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestStaticPathsは引数なしのパスヘルパーがcmd/groobb/serve.goで登録された
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
		{name: "AdminPath", got: templates.AdminPath(), want: "/admin"},
		{name: "AdminUsersPath", got: templates.AdminUsersPath(), want: "/admin/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != tt.want {
				t.Errorf("%s = %q、期待値 = %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestPath_AbsoluteURLは、パスがインスタンスの公開ベースURLの下で名指される
// こと、そして自身のアドレスを教えられていないインスタンスでは、ホスト相対のURLでは
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
			name:    "ベースURLの下の絶対URLになる",
			path:    templates.BoardPath("jazz"),
			baseURL: "https://groobb.example.com",
			want:    "https://groobb.example.com/b/jazz",
		},
		{
			name:    "ベースURLが空なら空になる",
			path:    templates.BoardPath("jazz"),
			baseURL: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.path.AbsoluteURL(tt.baseURL); got != tt.want {
				t.Errorf("AbsoluteURL(%q) = %q、期待値 = %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

// TestPath_WithReturnToは遷移先がエンコードされたreturn_toクエリパラメータとして
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
				t.Errorf("WithReturnTo(%q) = %q、期待値 = %q", tt.returnTo, got, tt.want)
			}
		})
	}
}

// TestAfterSignInPathは、セッションを発行するルートが、フローの運んできた遷移先へ、
// 運んでこなかったときはホームへ訪問者を着地させることを検証します。トップページではなく
// ホームであることで、サインインが2段リダイレクトにならない (トップページはサインイン済みの
// 訪問者をホームへ送るだけであるため)。
func TestAfterSignInPath(t *testing.T) {
	t.Parallel()

	if got := templates.AfterSignInPath("/settings"); got != "/settings" {
		t.Errorf("AfterSignInPath(%q) = %q、期待値 = %q", "/settings", got, "/settings")
	}
	if got := templates.AfterSignInPath(""); got != templates.HomePath() {
		t.Errorf("AfterSignInPath(%q) = %q、期待値 = %q", "", got, templates.HomePath())
	}
}

// TestCategoryPathはカテゴリーのslugが、カテゴリーのルートが登録されている
// /cの接頭辞の下に置かれることを検証します。サイドバーのリンクとルートが乖離しない
// ためです。
func TestCategoryPath(t *testing.T) {
	t.Parallel()

	if got, want := templates.CategoryPath("music"), templates.Path("/c/music"); got != want {
		t.Errorf("CategoryPath(%q) = %q、期待値 = %q", "music", got, want)
	}
}

// TestBoardPathは掲示板のslugが、掲示板のルートが登録されている /bの接頭辞の
// 下に置かれることを検証します。サイドバーのリンクとルートが乖離しないためです。
func TestBoardPath(t *testing.T) {
	t.Parallel()

	if got, want := templates.BoardPath("jazz"), templates.Path("/b/jazz"); got != want {
		t.Errorf("BoardPath(%q) = %q、期待値 = %q", "jazz", got, want)
	}
}

// TestPostElementIDはレス番号が、その投稿が描画される際のidになることを検証
// します。#p12で終わるアンカーがページ上で見つけねばならないものがこれであるためです。
func TestPostElementID(t *testing.T) {
	t.Parallel()

	if got, want := templates.PostElementID(12), "p12"; got != want {
		t.Errorf("PostElementID(%d) = %q、期待値 = %q", 12, got, want)
	}
}

// TestPostAnchorは投稿へのリンクが、その投稿のidへの同一文書内の参照になること
// を検証します。本文の >>NがPostElementIDの名指す要素へ繋がるようにするためです。
func TestPostAnchor(t *testing.T) {
	t.Parallel()

	if got, want := templates.PostAnchor(12), templates.Path("#p12"); got != want {
		t.Errorf("PostAnchor(%d) = %q、期待値 = %q", 12, got, want)
	}
}

// TestAdminUsersPagePathは、一覧の2つのパラメータのどちらがアドレスに現れるかを
// 検証します。何かを検索しているときの検索と、最初のページでないときのページ番号です。
// 何も述べない場面で双方を落とすことが、開いた一覧・検索した一覧・それぞれの下のページを、
// 1つずつのアドレスに保ちます。
func TestAdminUsersPagePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		atnamePrefix string
		page         int
		want         templates.Path
	}{
		{name: "絞り込みもページ番号も無い", atnamePrefix: "", page: 1, want: "/admin/users"},
		{name: "絞り込みのみ", atnamePrefix: "ali", page: 1, want: "/admin/users?q=ali"},
		{name: "ページ番号のみ", atnamePrefix: "", page: 2, want: "/admin/users?page=2"},
		{name: "絞り込みとページ番号", atnamePrefix: "ali", page: 3, want: "/admin/users?page=3&q=ali"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := templates.AdminUsersPagePath(tt.atnamePrefix, tt.page); got != tt.want {
				t.Errorf("AdminUsersPagePath(%q, %d) = %q、期待値 = %q", tt.atnamePrefix, tt.page, got, tt.want)
			}
		})
	}
}

// TestAdminUserRolePathsは、行のフォームが送信する2つのアドレスを検証します。
// 付与が書き込まれる先であるアカウントのロールと、剥奪が取り除く、そのアカウントが持つ
// 1つのロールです。2つ目が1つ目から組み立てられること、そしてどちらも
// cmd/groobb/serve.goで登録されたルートと一致する必要があることから、まとめて検証します。
func TestAdminUserRolePaths(t *testing.T) {
	t.Parallel()

	id := viewmodel.UserID(42)

	if got, want := templates.AdminUserRolesPath(id), templates.Path("/admin/users/42/roles"); got != want {
		t.Errorf("AdminUserRolesPath(%v) = %q、期待値 = %q", id, got, want)
	}
	if got, want := templates.AdminUserRolePath(id, "admin"), templates.Path("/admin/users/42/roles/admin"); got != want {
		t.Errorf("AdminUserRolePath(%v, %q) = %q、期待値 = %q", id, "admin", got, want)
	}
}

// TestModerationPathsは、モデレーションの各画面が到達されるアドレスを検証します。
// スレッドや投稿が持つ印、それぞれを確認するページ、そしてスレッドではないページからの投稿
// 1件へのリンクです。投稿の2つはスレッドの中でそれを名指す組から組み立てられるため
// (ADR 0009)、番号がパスの他のどこでもなくスレッドの投稿と印の間に来ることをここで述べます。
func TestModerationPaths(t *testing.T) {
	t.Parallel()

	const id = viewmodel.ThreadID(12)
	const number = 3

	tests := []struct {
		name string
		got  templates.Path
		want templates.Path
	}{
		{name: "スレッドのロック", got: templates.ThreadLockPath(id), want: "/t/12/lock"},
		{name: "スレッドのロックの確認", got: templates.ThreadLockNewPath(id), want: "/t/12/lock/new"},
		{name: "スレッドの非公開", got: templates.ThreadUnpublicationPath(id), want: "/t/12/unpublication"},
		{name: "スレッドの非公開の確認", got: templates.ThreadUnpublicationNewPath(id), want: "/t/12/unpublication/new"},
		{name: "投稿の非公開", got: templates.PostUnpublicationPath(id, number), want: "/t/12/posts/3/unpublication"},
		{name: "投稿の非公開の確認", got: templates.PostUnpublicationNewPath(id, number), want: "/t/12/posts/3/unpublication/new"},
		{name: "別のページからの投稿へのアンカー", got: templates.ThreadPostAnchorPath(id, number), want: "/t/12#p3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != tt.want {
				t.Errorf("%s = %q、期待値 = %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
