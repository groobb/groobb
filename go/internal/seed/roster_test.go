package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/testutil"
)

// withdrawnEntry is the entry for the role a run creates last, kept apart so
// that a test can take it back out of the roster to leave a role the generators
// name unfilled.
//
// [Ja] withdrawnEntry は、実行が最後に作成する役割の項目です。テストが名簿からこれを
// 取り除き、生成器が名指しする役割が埋まっていない名簿を作れるよう、別に切り出して
// います。
const withdrawnEntry = `
[[users]]
role = "withdrawn"
atname = "seeduser3"
email = "seeduser3@example.com"
note = "withdraws, leaving its posts behind"
`

// rosterUsers is the [[users]] section of a roster that passes every check: one
// entry for each role the generators name.
//
// [Ja] rosterUsers は、すべての検査を通る名簿の [[users]] 部分です。生成器が名指しする
// 役割それぞれに 1 件ずつ対応します。
const rosterUsers = `
[[users]]
role = "starter"
atname = "seeduser1"
email = "seeduser1@example.com"
note = "opens the threads a board lists"

[[users]]
role = "replier"
atname = "seeduser2"
email = "seeduser2@example.com"
note = "replies to them"
` + withdrawnEntry

// validRosterPassword is the password validRoster shares between its accounts.
//
// [Ja] validRosterPassword は、validRoster のアカウントが共有するパスワードです。
const validRosterPassword = "seed-password"

// validRoster is a roster that passes every check, which the tests below break
// one part of at a time.
//
// [Ja] validRoster はすべての検査を通る名簿です。以下のテストは、これを 1 箇所ずつ
// 壊して確認します。
var validRoster = rosterWithPassword(validRosterPassword)

// rosterWithPassword returns a complete roster whose accounts share password.
// A test that has to know the password, rather than only that the roster is a
// valid one, asks for the roster by the password it wants.
//
// [Ja] rosterWithPassword は、アカウントが password を共有する完全な名簿を返します。
// 名簿が正しいことだけでなくパスワードそのものを知る必要のあるテストは、欲しい
// パスワードを指定して名簿を受け取ります。
func rosterWithPassword(password string) string {
	return `password = "` + password + `"
` + rosterUsers
}

// writeRoster writes content to a roster file of the test's own and returns its
// path.
//
// [Ja] writeRoster は content をテスト専用の名簿ファイルへ書き、そのパスを返します。
func writeRoster(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), rosterPath)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write the roster: %v", err)
	}

	return path
}

// TestLoadUserRoster verifies that a valid roster comes back as the accounts the
// generators work from, in the order the file writes them, with the shared
// password already hashed.
//
// [Ja] TestLoadUserRoster は、正しい名簿が、生成器の使う形のアカウントとして、ファイルが
// 書いた順のまま、共通パスワードをハッシュ化済みの状態で返ることを検証します。
func TestLoadUserRoster(t *testing.T) {
	t.Parallel()

	testutil.LowerBcryptCost()

	path := writeRoster(t, validRoster)

	roster, err := loadUserRoster(path)
	if err != nil {
		t.Fatalf("loadUserRoster() error = %v", err)
	}

	// The path travels with the roster because a run reports which file it read.
	//
	// [Ja] パスを名簿と一緒に持つのは、実行がどのファイルを読んだのかを報告するためです。
	if roster.path != path {
		t.Errorf("roster.path = %q, want %q", roster.path, path)
	}

	// What a run writes is the digest, so checking it against the password
	// written in the file is what says the file's password was read.
	//
	// [Ja] 実行が書き込むのはダイジェストであるため、ファイルに書いたパスワードでそれを
	// 検証することが、ファイルのパスワードが読めていることの確認になります。
	if err := auth.CheckPassword(roster.passwordDigest, validRosterPassword); err != nil {
		t.Errorf("the password digest does not match the password in the roster: %v", err)
	}

	want := []rosterUser{
		{role: roleStarter, atname: "seeduser1", email: "seeduser1@example.com", note: "opens the threads a board lists"},
		{role: roleReplier, atname: "seeduser2", email: "seeduser2@example.com", note: "replies to them"},
		{role: roleWithdrawn, atname: "seeduser3", email: "seeduser3@example.com", note: "withdraws, leaving its posts behind"},
	}
	if len(roster.users) != len(want) {
		t.Fatalf("len(roster.users) = %d, want %d", len(roster.users), len(want))
	}
	for i, wantUser := range want {
		if roster.users[i] != wantUser {
			t.Errorf("roster.users[%d] = %+v, want %+v", i, roster.users[i], wantUser)
		}
	}
}

// TestLoadUserRoster_TrimsTheNote verifies that whitespace around a note is
// dropped, so that a stray space in the file does not become a report line that
// reads as indented.
//
// [Ja] TestLoadUserRoster_TrimsTheNote は、覚え書きの前後の空白が落ちることを検証します。
// ファイルに紛れ込んだ空白が、字下げされて見える報告行にならないようにするためです。
func TestLoadUserRoster_TrimsTheNote(t *testing.T) {
	t.Parallel()

	testutil.LowerBcryptCost()

	path := writeRoster(t, strings.Replace(
		validRoster,
		`note = "opens the threads a board lists"`,
		`note = "  opens the threads a board lists  "`,
		1,
	))

	roster, err := loadUserRoster(path)
	if err != nil {
		t.Fatalf("loadUserRoster() error = %v", err)
	}
	if roster.users[0].note != "opens the threads a board lists" {
		t.Errorf("roster.users[0].note = %q, want it trimmed", roster.users[0].note)
	}
}

// TestLoadUserRoster_RejectsAMissingFile verifies that an absent roster is an
// error naming the example to copy, rather than a silent fall back to accounts
// nobody reads mail for.
//
// [Ja] TestLoadUserRoster_RejectsAMissingFile は、名簿が無いことが、複製すべき見本を
// 名指しするエラーになり、誰もメールを読まないアカウントへ黙ってフォールバックしない
// ことを検証します。
func TestLoadUserRoster_RejectsAMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), rosterPath)

	_, err := loadUserRoster(path)
	if err == nil {
		t.Fatal("loadUserRoster() should fail when the roster does not exist, but it succeeded")
	}
	if !strings.Contains(err.Error(), rosterExamplePath) {
		t.Errorf("loadUserRoster() error = %q, want it to name %q", err, rosterExamplePath)
	}
}

// TestLoadUserRoster_RejectsASyntaxErrorWithoutQuotingTheFile verifies that a
// roster the parser cannot read is reported by position alone. The error is
// logged, and the parser's own message quotes the token it stumbled on, which
// on the password line is the password every account signs in with.
//
// [Ja] TestLoadUserRoster_RejectsASyntaxErrorWithoutQuotingTheFile は、パーサーが
// 読めない名簿が位置だけで報告されることを検証します。このエラーはログへ出るうえ、
// パーサー自身のメッセージはつまずいたトークンを引用するため、それが password の行で
// あれば、すべてのアカウントがサインインに使うパスワードが引用されます。
func TestLoadUserRoster_RejectsASyntaxErrorWithoutQuotingTheFile(t *testing.T) {
	t.Parallel()

	const secret = "unquoted-secret-password"
	path := writeRoster(t, strings.Replace(
		validRoster,
		`password = "`+validRosterPassword+`"`,
		`password = `+secret,
		1,
	))

	_, err := loadUserRoster(path)
	if err == nil {
		t.Fatal("loadUserRoster() should fail on a roster the parser cannot read, but it succeeded")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("loadUserRoster() error = %q, want it not to quote the password", err)
	}
	for _, want := range []string{"line 1", `last key "password"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("loadUserRoster() error = %q, want it to contain %q", err, want)
		}
	}
}

// TestLoadUserRoster_RejectsAnInvalidRoster verifies that a roster is checked
// over before the database is touched: a required value left out, a value the
// generators cannot use, and a value written twice all stop the run.
//
// [Ja] TestLoadUserRoster_RejectsAnInvalidRoster は、データベースへ触れる前に名簿が
// 検査されることを検証します。必須の値の欠落・生成器が使えない値・2 度書かれた値の
// いずれもが実行を止めます。
func TestLoadUserRoster_RejectsAnInvalidRoster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roster       string
		wantContains string
	}{
		{
			name:         "empty password",
			roster:       strings.Replace(validRoster, `password = "`+validRosterPassword+`"`, `password = ""`, 1),
			wantContains: "password is empty",
		},
		{
			name:         "password holding a line break",
			roster:       strings.Replace(validRoster, `password = "`+validRosterPassword+`"`, `password = "seed\npassword"`, 1),
			wantContains: "password cannot contain CR or LF",
		},
		{
			name:         "no accounts",
			roster:       `password = "` + validRosterPassword + `"`,
			wantContains: "there is no [[users]] entry",
		},
		{
			name:         "missing role",
			roster:       strings.Replace(validRoster, "role = \"starter\"\n", "", 1),
			wantContains: "[[users]] entry 1: role is empty",
		},
		{
			name:         "missing note",
			roster:       strings.Replace(validRoster, "note = \"opens the threads a board lists\"\n", "", 1),
			wantContains: "note is empty",
		},
		{
			name:         "role the generators do not know",
			roster:       strings.Replace(validRoster, `role = "starter"`, `role = "lurker"`, 1),
			wantContains: `the role "lurker" is not one the generators know`,
		},
		{
			name:         "atname outside the allowed format",
			roster:       strings.Replace(validRoster, `atname = "seeduser1"`, `atname = "seed user1"`, 1),
			wantContains: "may hold only ASCII letters, digits and underscores",
		},
		{
			name:         "email that is not an address",
			roster:       strings.Replace(validRoster, `email = "seeduser1@example.com"`, `email = "not-an-email"`, 1),
			wantContains: "email is not an email address",
		},
		{
			name:         "email carrying a display name",
			roster:       strings.Replace(validRoster, `email = "seeduser1@example.com"`, `email = "Seed User <seeduser1@example.com>"`, 1),
			wantContains: "email must hold the address alone",
		},
		{
			name:         "role written twice",
			roster:       strings.Replace(validRoster, `role = "replier"`, `role = "starter"`, 1),
			wantContains: "there is more than one [[users]] entry with the role starter",
		},
		{
			name:         "atname written twice in different letter case",
			roster:       strings.Replace(validRoster, `atname = "seeduser2"`, `atname = "SEEDUSER1"`, 1),
			wantContains: "there is more than one [[users]] entry with the atname",
		},
		{
			name:         "email written twice in different letter case",
			roster:       strings.Replace(validRoster, `email = "seeduser2@example.com"`, `email = "SEEDUSER1@example.com"`, 1),
			wantContains: "there is more than one [[users]] entry with the email",
		},
		{
			name:         "role the generators name left unfilled",
			roster:       strings.Replace(validRoster, withdrawnEntry, "", 1),
			wantContains: "there is no [[users]] entry with the role withdrawn",
		},
		{
			name:         "key that does not exist",
			roster:       strings.Replace(validRoster, `note = "replies to them"`, "note = \"replies to them\"\nnickname = \"seed\"", 1),
			wantContains: "keys that do not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeRoster(t, tt.roster)

			_, err := loadUserRoster(path)
			if err == nil {
				t.Fatal("loadUserRoster() should fail on this roster, but it succeeded")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("loadUserRoster() error = %q, want it to contain %q", err, tt.wantContains)
			}
		})
	}
}

// TestLoadUserRoster_AcceptsTheExampleFile verifies that the example committed
// for developers to copy is one the loader accepts. An example that does not
// load would hand whoever copied it an error instead of a seeded database.
//
// [Ja] TestLoadUserRoster_AcceptsTheExampleFile は、開発者が複製するためにコミットして
// いる見本を、読み込み側が受理することを検証します。読み込めない見本は、それを複製した
// 人にシード済みのデータベースではなくエラーを渡すことになります。
func TestLoadUserRoster_AcceptsTheExampleFile(t *testing.T) {
	t.Parallel()

	testutil.LowerBcryptCost()

	if _, err := loadUserRoster(filepath.Join("..", "..", rosterExamplePath)); err != nil {
		t.Errorf("loadUserRoster(%s) error = %v", rosterExamplePath, err)
	}
}
