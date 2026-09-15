package seed

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestFindCredentialsは、役割が、ブラウザ確認がサインインに使うアドレスと、名簿が
// アカウント間で共有しているパスワードとして返ることを検証します。
func TestFindCredentials(t *testing.T) {
	t.Parallel()

	path := writeRoster(t, validRoster)

	tests := []struct {
		name      string
		role      seedRole
		wantEmail string
	}{
		{name: "starter", role: roleStarter, wantEmail: "seeduser1@example.com"},
		{name: "replier", role: roleReplier, wantEmail: "seeduser2@example.com"},
		{name: "admin", role: roleAdmin, wantEmail: "seeduser4@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			credentials, err := findCredentials(path, string(tt.role))
			if err != nil {
				t.Fatalf("findCredentials()のエラー = %v", err)
			}

			if credentials.Email != tt.wantEmail {
				t.Errorf("credentials.Email = %q、期待値 = %q", credentials.Email, tt.wantEmail)
			}

			// パスワードは全アカウント共通であるため、ここで確認しているのは、実行が
			// 書き込むダイジェストではなく名簿の平文が返ることです。
			if credentials.Password != validRosterPassword {
				t.Errorf("credentials.Password = %q、期待値 = %q", credentials.Password, validRosterPassword)
			}
		})
	}
}

// TestFindCredentials_RejectsAWithdrawnRoleは、作者のいないコンテンツを生成するための
// アカウントを、ブラウザがサインインできるものとして示さないことを検証します。この
// アカウントはシード実行の完了前に論理削除されるため、その時点で名簿のアドレスは履歴であり、
// 使用可能な資格情報ではありません。
func TestFindCredentials_RejectsAWithdrawnRole(t *testing.T) {
	t.Parallel()

	_, err := findCredentials(writeRoster(t, validRoster), string(roleWithdrawn))
	if err == nil {
		t.Fatal("退会済みロールに対してfindCredentials()が失敗することを期待したが、成功した")
	}
	if !strings.Contains(err.Error(), "does not name an account that can sign in after seeding") {
		t.Errorf("findCredentials()のエラー = %q、そのロールを使えない理由の説明を期待", err)
	}
}

// TestFindCredentials_RejectsAnUnknownRoleは、役割の書き間違いに対して、シード完了後も
// サインインできる役割を案内することを検証します。withdrawnも生成器の役割ですが、この
// 一覧へ含めると、完了したシードが既に無効化したアカウントの指定を促すことになります。
func TestFindCredentials_RejectsAnUnknownRole(t *testing.T) {
	t.Parallel()

	_, err := findCredentials(writeRoster(t, validRoster), "startr")
	if err == nil {
		t.Fatal("名簿に無いロールに対してfindCredentials()が失敗することを期待したが、成功した")
	}

	for _, role := range signInSeedRoles {
		if !strings.Contains(err.Error(), string(role)) {
			t.Errorf("findCredentials()のエラー = %q、サインイン用のロール %q を挙げることを期待", err, role)
		}
	}
	if strings.Contains(err.Error(), string(roleWithdrawn)) {
		t.Errorf("findCredentials()のエラー = %q、退会済みロールを案内しないことを期待", err)
	}
}

// TestFindCredentials_RejectsAnInvalidRosterは、尋ねられた1件だけでなく名簿全体が
// 検査されることを検証します。ブラウザ確認がサインインするのはシードが作成したアカウント
// であり、シードが拒否する名簿には、そのアカウントが存在しません。
func TestFindCredentials_RejectsAnInvalidRoster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roster       string
		wantContains string
	}{
		{
			name:         "尋ねたものとは別のロールが2度書かれている",
			roster:       strings.Replace(validRoster, `role = "withdrawn"`, `role = "replier"`, 1),
			wantContains: "more than one [[users]] entry with the role replier",
		},
		{
			name:         "存在しないキー",
			roster:       strings.Replace(validRoster, "atname = ", "atnam = ", 1),
			wantContains: "keys that do not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := findCredentials(writeRoster(t, tt.roster), string(roleStarter))
			if err == nil {
				t.Fatal("シードが拒否する名簿に対してfindCredentials()が失敗することを期待したが、成功した")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("findCredentials()のエラー = %q、%q を含むことを期待", err, tt.wantContains)
			}
		})
	}
}

// TestFindCredentials_RejectsAMissingFileは、名簿が無いときに複製すべき見本が
// 名指しされることを検証します。名簿を用意していない開発者は、シードからと同じくブラウザ
// 確認からもこのエラーに出会うため、どちらから来ても同じファイルを案内する必要があります。
func TestFindCredentials_RejectsAMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), rosterPath)

	_, err := findCredentials(path, string(roleStarter))
	if err == nil {
		t.Fatal("名簿が存在しないときにfindCredentials()が失敗することを期待したが、成功した")
	}
	if !strings.Contains(err.Error(), rosterExamplePath) {
		t.Errorf("findCredentials()のエラー = %q、%q を名指すことを期待", err, rosterExamplePath)
	}
}
