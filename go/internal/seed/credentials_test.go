package seed

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestFindCredentials verifies that a role comes back as the address the
// browser verification signs in with, together with the password the roster
// shares between its accounts.
//
// [Ja] TestFindCredentials は、役割が、ブラウザ確認がサインインに使うアドレスと、名簿が
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			credentials, err := findCredentials(path, string(tt.role))
			if err != nil {
				t.Fatalf("findCredentials() error = %v", err)
			}

			if credentials.Email != tt.wantEmail {
				t.Errorf("credentials.Email = %q, want %q", credentials.Email, tt.wantEmail)
			}

			// The password is shared by every account, so what this checks is
			// that the plaintext of the roster comes back rather than the digest
			// a run writes.
			//
			// [Ja] パスワードは全アカウント共通であるため、ここで確認しているのは、実行が
			// 書き込むダイジェストではなく名簿の平文が返ることです。
			if credentials.Password != validRosterPassword {
				t.Errorf("credentials.Password = %q, want %q", credentials.Password, validRosterPassword)
			}
		})
	}
}

// TestFindCredentials_RejectsAWithdrawnRole verifies that an account used to
// generate authorless content is not presented as one the browser can sign in
// as. The account is soft-deleted before a seeding run finishes, so the address
// in the roster is history rather than a usable credential by then.
//
// [Ja] TestFindCredentials_RejectsAWithdrawnRole は、作者のいないコンテンツを生成するための
// アカウントを、ブラウザがサインインできるものとして示さないことを検証します。この
// アカウントはシード実行の完了前に論理削除されるため、その時点で名簿のアドレスは履歴であり、
// 使用可能な資格情報ではありません。
func TestFindCredentials_RejectsAWithdrawnRole(t *testing.T) {
	t.Parallel()

	_, err := findCredentials(writeRoster(t, validRoster), string(roleWithdrawn))
	if err == nil {
		t.Fatal("findCredentials() should fail on the withdrawn role, but it succeeded")
	}
	if !strings.Contains(err.Error(), "does not name an account that can sign in after seeding") {
		t.Errorf("findCredentials() error = %q, want it to explain why the role cannot be used", err)
	}
}

// TestFindCredentials_RejectsAnUnknownRole verifies that a misspelling is
// answered with the roles that remain able to sign in. The withdrawn role is a
// generator role too, but including it in this list would invite an invocation
// whose account the completed seed has already disabled.
//
// [Ja] TestFindCredentials_RejectsAnUnknownRole は、役割の書き間違いに対して、シード完了後も
// サインインできる役割を案内することを検証します。withdrawn も生成器の役割ですが、この
// 一覧へ含めると、完了したシードが既に無効化したアカウントの指定を促すことになります。
func TestFindCredentials_RejectsAnUnknownRole(t *testing.T) {
	t.Parallel()

	_, err := findCredentials(writeRoster(t, validRoster), "startr")
	if err == nil {
		t.Fatal("findCredentials() should fail on a role the roster does not hold, but it succeeded")
	}

	for _, role := range signInSeedRoles {
		if !strings.Contains(err.Error(), string(role)) {
			t.Errorf("findCredentials() error = %q, want it to list the sign-in role %q", err, role)
		}
	}
	if strings.Contains(err.Error(), string(roleWithdrawn)) {
		t.Errorf("findCredentials() error = %q, want it not to offer the withdrawn role", err)
	}
}

// TestFindCredentials_RejectsAnInvalidRoster verifies that the whole roster is
// looked over, not only the entry that was asked for. The browser verification
// signs in as an account the seed created, and a roster the seed would refuse
// holds no such account.
//
// [Ja] TestFindCredentials_RejectsAnInvalidRoster は、尋ねられた 1 件だけでなく名簿全体が
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
			name:         "a role other than the one asked for is written twice",
			roster:       strings.Replace(validRoster, `role = "withdrawn"`, `role = "replier"`, 1),
			wantContains: "more than one [[users]] entry with the role replier",
		},
		{
			name:         "a key that does not exist",
			roster:       strings.Replace(validRoster, "atname = ", "atnam = ", 1),
			wantContains: "keys that do not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := findCredentials(writeRoster(t, tt.roster), string(roleStarter))
			if err == nil {
				t.Fatal("findCredentials() should fail on a roster the seed would refuse, but it succeeded")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("findCredentials() error = %q, want it to contain %q", err, tt.wantContains)
			}
		})
	}
}

// TestFindCredentials_RejectsAMissingFile verifies that an absent roster names
// the example to copy. A developer who has not set the roster up meets this
// error through the browser verification as much as through the seed, so it has
// to point at the same file either way.
//
// [Ja] TestFindCredentials_RejectsAMissingFile は、名簿が無いときに複製すべき見本が
// 名指しされることを検証します。名簿を用意していない開発者は、シードからと同じくブラウザ
// 確認からもこのエラーに出会うため、どちらから来ても同じファイルを案内する必要があります。
func TestFindCredentials_RejectsAMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), rosterPath)

	_, err := findCredentials(path, string(roleStarter))
	if err == nil {
		t.Fatal("findCredentials() should fail when the roster does not exist, but it succeeded")
	}
	if !strings.Contains(err.Error(), rosterExamplePath) {
		t.Errorf("findCredentials() error = %q, want it to name %q", err, rosterExamplePath)
	}
}
