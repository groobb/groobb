package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/testutil"
)

// withdrawnEntryは、実行が最後に作成する役割の項目です。テストが名簿からこれを
// 取り除き、生成器が名指しする役割が埋まっていない名簿を作れるよう、別に切り出して
// います。
const withdrawnEntry = `
[[users]]
role = "withdrawn"
atname = "seeduser3"
email = "seeduser3@example.com"
note = "withdraws, leaving its posts behind"
`

// rosterUsersは、すべての検査を通る名簿の [[users]] 部分です。生成器が名指しする
// 役割それぞれに1件ずつ対応します。
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

[[users]]
role = "admin"
atname = "seeduser4"
email = "seeduser4@example.com"
note = "administers the community"
` + withdrawnEntry

// validRosterPasswordは、validRosterのアカウントが共有するパスワードです。
const validRosterPassword = "seed-password"

// validRosterはすべての検査を通る名簿です。以下のテストは、これを1箇所ずつ
// 壊して確認します。
var validRoster = rosterWithPassword(validRosterPassword)

// rosterWithPasswordは、アカウントがpasswordを共有する完全な名簿を返します。
// 名簿が正しいことだけでなくパスワードそのものを知る必要のあるテストは、欲しい
// パスワードを指定して名簿を受け取ります。
func rosterWithPassword(password string) string {
	return `password = "` + password + `"
` + rosterUsers
}

// writeRosterはcontentをテスト専用の名簿ファイルへ書き、そのパスを返します。
func writeRoster(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), rosterPath)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("名簿の書き込みに失敗: %v", err)
	}

	return path
}

// TestLoadUserRosterは、正しい名簿が、生成器の使う形のアカウントとして、ファイルが
// 書いた順のまま、共通パスワードをハッシュ化済みの状態で返ることを検証します。
func TestLoadUserRoster(t *testing.T) {
	t.Parallel()

	testutil.LowerBcryptCost()

	path := writeRoster(t, validRoster)

	roster, err := loadUserRoster(path)
	if err != nil {
		t.Fatalf("loadUserRoster()のエラー = %v", err)
	}

	// パスを名簿と一緒に持つのは、実行がどのファイルを読んだのかを報告するためです。
	if roster.path != path {
		t.Errorf("roster.path = %q、期待値 = %q", roster.path, path)
	}

	// 実行が書き込むのはダイジェストであるため、ファイルに書いたパスワードでそれを
	// 検証することが、ファイルのパスワードが読めていることの確認になります。
	if err := auth.CheckPassword(roster.passwordDigest, validRosterPassword); err != nil {
		t.Errorf("パスワードダイジェストが名簿のパスワードと一致しない: %v", err)
	}

	want := []rosterUser{
		{role: roleStarter, atname: "seeduser1", email: "seeduser1@example.com", note: "opens the threads a board lists"},
		{role: roleReplier, atname: "seeduser2", email: "seeduser2@example.com", note: "replies to them"},
		{role: roleAdmin, atname: "seeduser4", email: "seeduser4@example.com", note: "administers the community"},
		{role: roleWithdrawn, atname: "seeduser3", email: "seeduser3@example.com", note: "withdraws, leaving its posts behind"},
	}
	if len(roster.users) != len(want) {
		t.Fatalf("len(roster.users) = %d、期待値 = %d", len(roster.users), len(want))
	}
	for i, wantUser := range want {
		if roster.users[i] != wantUser {
			t.Errorf("roster.users[%d] = %+v、期待値 = %+v", i, roster.users[i], wantUser)
		}
	}
}

// TestLoadUserRoster_TrimsTheNoteは、覚え書きの前後の空白が落ちることを検証します。
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
		t.Fatalf("loadUserRoster()のエラー = %v", err)
	}
	if roster.users[0].note != "opens the threads a board lists" {
		t.Errorf("roster.users[0].note = %q、前後の空白が取り除かれることを期待", roster.users[0].note)
	}
}

// TestLoadUserRoster_RejectsAMissingFileは、名簿が無いことが、複製すべき見本を
// 名指しするエラーになり、誰もメールを読まないアカウントへ黙ってフォールバックしない
// ことを検証します。
func TestLoadUserRoster_RejectsAMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), rosterPath)

	_, err := loadUserRoster(path)
	if err == nil {
		t.Fatal("名簿が存在しないときにloadUserRoster()が失敗することを期待したが、成功した")
	}
	if !strings.Contains(err.Error(), rosterExamplePath) {
		t.Errorf("loadUserRoster()のエラー = %q、%q を名指すことを期待", err, rosterExamplePath)
	}
}

// TestLoadUserRoster_RejectsASyntaxErrorWithoutQuotingTheFileは、パーサーが
// 読めない名簿が位置だけで報告されることを検証します。このエラーはログへ出るうえ、
// パーサー自身のメッセージはつまずいたトークンを引用するため、それがpasswordの行で
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
		t.Fatal("パーサーが読めない名簿に対してloadUserRoster()が失敗することを期待したが、成功した")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("loadUserRoster()のエラー = %q、パスワードを引用しないことを期待", err)
	}
	for _, want := range []string{"line 1", `last key "password"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("loadUserRoster()のエラー = %q、%q を含むことを期待", err, want)
		}
	}
}

// TestLoadUserRoster_RejectsAnInvalidRosterは、データベースへ触れる前に名簿が
// 検査されることを検証します。必須の値の欠落・生成器が使えない値・2度書かれた値の
// いずれもが実行を止めます。
func TestLoadUserRoster_RejectsAnInvalidRoster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roster       string
		wantContains string
	}{
		{
			name:         "パスワードが空",
			roster:       strings.Replace(validRoster, `password = "`+validRosterPassword+`"`, `password = ""`, 1),
			wantContains: "password is empty",
		},
		{
			name:         "パスワードに改行を含む",
			roster:       strings.Replace(validRoster, `password = "`+validRosterPassword+`"`, `password = "seed\npassword"`, 1),
			wantContains: "password cannot contain CR or LF",
		},
		{
			name:         "アカウントが無い",
			roster:       `password = "` + validRosterPassword + `"`,
			wantContains: "there is no [[users]] entry",
		},
		{
			name:         "roleが無い",
			roster:       strings.Replace(validRoster, "role = \"starter\"\n", "", 1),
			wantContains: "[[users]] entry 1: role is empty",
		},
		{
			name:         "noteが無い",
			roster:       strings.Replace(validRoster, "note = \"opens the threads a board lists\"\n", "", 1),
			wantContains: "note is empty",
		},
		{
			name:         "生成器が知らないrole",
			roster:       strings.Replace(validRoster, `role = "starter"`, `role = "lurker"`, 1),
			wantContains: `the role "lurker" is not one the generators know`,
		},
		{
			name:         "許される形式から外れたatname",
			roster:       strings.Replace(validRoster, `atname = "seeduser1"`, `atname = "seed user1"`, 1),
			wantContains: "may hold only ASCII letters, digits and underscores",
		},
		{
			name:         "アドレスでないemail",
			roster:       strings.Replace(validRoster, `email = "seeduser1@example.com"`, `email = "not-an-email"`, 1),
			wantContains: "email is not an email address",
		},
		{
			name:         "表示名付きのemail",
			roster:       strings.Replace(validRoster, `email = "seeduser1@example.com"`, `email = "Seed User <seeduser1@example.com>"`, 1),
			wantContains: "email must hold the address alone",
		},
		{
			name:         "roleが2度書かれている",
			roster:       strings.Replace(validRoster, `role = "replier"`, `role = "starter"`, 1),
			wantContains: "there is more than one [[users]] entry with the role starter",
		},
		{
			name:         "大文字小文字違いでatnameが2度書かれている",
			roster:       strings.Replace(validRoster, `atname = "seeduser2"`, `atname = "SEEDUSER1"`, 1),
			wantContains: "there is more than one [[users]] entry with the atname",
		},
		{
			name:         "大文字小文字違いでemailが2度書かれている",
			roster:       strings.Replace(validRoster, `email = "seeduser2@example.com"`, `email = "SEEDUSER1@example.com"`, 1),
			wantContains: "there is more than one [[users]] entry with the email",
		},
		{
			name:         "生成器が名指すroleが埋まっていない",
			roster:       strings.Replace(validRoster, withdrawnEntry, "", 1),
			wantContains: "there is no [[users]] entry with the role withdrawn",
		},
		{
			name:         "存在しないキー",
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
				t.Fatal("この名簿に対してloadUserRoster()が失敗することを期待したが、成功した")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("loadUserRoster()のエラー = %q、%q を含むことを期待", err, tt.wantContains)
			}
		})
	}
}

// TestLoadUserRoster_AcceptsTheExampleFileは、開発者が複製するためにコミットして
// いる見本を、読み込み側が受理することを検証します。読み込めない見本は、それを複製した
// 人にシード済みのデータベースではなくエラーを渡すことになります。
func TestLoadUserRoster_AcceptsTheExampleFile(t *testing.T) {
	t.Parallel()

	testutil.LowerBcryptCost()

	if _, err := loadUserRoster(filepath.Join("..", "..", rosterExamplePath)); err != nil {
		t.Errorf("loadUserRoster(%s)のエラー = %v", rosterExamplePath, err)
	}
}
