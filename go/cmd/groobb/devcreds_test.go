package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/seed"
)

// stubCredentials is what a lookup that succeeds hands back in these tests.
//
// [Ja] stubCredentials は、これらのテストで成功する引きが返す値です。
var stubCredentials = &seed.Credentials{Email: "starter@example.com", Password: "seed-password"}

// TestRun_DispatchesDevCredentialsUsage verifies that the top-level command
// removes the subcommand name before handing the remaining arguments to
// devcreds. An invocation naming no role is answered before the roster is read,
// so this part of the wiring is observable without a roster on disk.
//
// [Ja] TestRun_DispatchesDevCredentialsUsage は、トップレベルコマンドがサブコマンド名を
// 取り除き、残りの引数を devcreds へ渡すことを検証します。役割を指定しない実行は名簿を
// 読む前に応答されるため、この配線はディスク上の名簿無しで観測できます。
func TestRun_DispatchesDevCredentialsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"devcreds"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run() exit code = %d, want %d", code, exitUsage)
	}
	if want := "usage: groobb devcreds starter|replier\n"; stderr.String() != want {
		t.Errorf("run() stderr = %q, want %q", stderr.String(), want)
	}
}

// TestRun_DispatchesDevCredentialsOutput verifies the successful path from the
// top-level dispatch to the standard output a caller supplied. It uses a roster
// in a temporary working directory so neither the developer's untracked roster
// nor its credentials become a test dependency.
//
// This test is intentionally sequential: both t.Chdir and t.Setenv change
// process-wide state, and Go prevents their use after t.Parallel for that
// reason. Other parallel tests in this package remain paused while it runs.
//
// [Ja] TestRun_DispatchesDevCredentialsOutput は、トップレベルの振り分けから呼び出し側が
// 渡した標準出力までの成功経路を検証します。一時作業ディレクトリの名簿を使うことで、
// 開発者の追跡対象外の名簿とその資格情報をテストの依存にしません。
//
// 本テストは意図的に逐次実行します。t.Chdir と t.Setenv はどちらもプロセス全体の状態を
// 変更するため、Go も t.Parallel の後での利用を禁止しています。実行中は本パッケージの
// 他の並列テストが待機します。
func TestRun_DispatchesDevCredentialsOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("APP_ENV", "dev")

	const roster = `password = "seed-password"

[[users]]
role = "starter"
atname = "seeduser1"
email = "starter@example.com"
note = "starts threads"

[[users]]
role = "replier"
atname = "seeduser2"
email = "replier@example.com"
note = "replies to threads"

[[users]]
role = "withdrawn"
atname = "seeduser3"
email = "withdrawn@example.com"
note = "leaves authorless posts"
`
	if err := os.WriteFile("seed-users.toml", []byte(roster), 0o600); err != nil {
		t.Fatalf("failed to write the isolated roster: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"devcreds", "starter"}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("run() exit code = %d, want 0", code)
	}
	if want := "starter@example.com\nseed-password\n"; stdout.String() != want {
		t.Errorf("run() stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("run() stderr = %q, want it to be empty", stderr.String())
	}
}

// TestRunDevCredentials_RejectsAWrongArgumentCount covers the invocations that
// name no role and the ones that name more than one. A shell reads this
// command's standard output as the credentials themselves, so an invocation it
// cannot answer has to leave that stream empty and say what happened on standard
// error.
//
// [Ja] TestRunDevCredentials_RejectsAWrongArgumentCount は、役割を指定していない実行と、
// 2 つ以上指定した実行を扱います。シェルは本コマンドの標準出力を資格情報そのものとして
// 読むため、答えられない実行はその出力を空のままにし、何が起きたのかは標準エラー出力で
// 告げる必要があります。
func TestRunDevCredentials_RejectsAWrongArgumentCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "no role", args: []string{}},
		{name: "more than one role", args: []string{"starter", "replier"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			lookedUp := false
			code := runDevCredentials(t.Context(), tt.args, "dev", func(string) (*seed.Credentials, error) {
				lookedUp = true

				return stubCredentials, nil
			}, &stdout, &stderr)

			if code != exitUsage {
				t.Errorf("runDevCredentials() exit code = %d, want %d", code, exitUsage)
			}
			if stdout.Len() != 0 {
				t.Errorf("runDevCredentials() stdout = %q, want it to be empty", stdout.String())
			}
			if want := "usage: groobb devcreds starter|replier"; !strings.Contains(stderr.String(), want) {
				t.Errorf("runDevCredentials() stderr = %q, want it to contain %q", stderr.String(), want)
			}

			// A role is what the lookup is asked for, so an invocation naming
			// none, or naming two, has nothing to look up. Checking that it does
			// not is what says the argument check stands in front of the roster,
			// rather than leaving the answer to whether that file happens to be
			// readable from wherever the command was started.
			//
			// [Ja] 引きが尋ねられるのは役割であるため、役割を指定していない実行と 2 つ
			// 指定した実行には、引くものがありません。引かないことを確認することが、引数の
			// 検査が名簿の手前に立っていることを言う方法になります。そうしなければ、その
			// 答えは、コマンドを開始した場所からそのファイルが読めるかどうかに委ねられます。
			if lookedUp {
				t.Error("runDevCredentials() looked the credentials up, want it to answer the argument count first")
			}
		})
	}
}

// TestRunDevCredentials_WritesTheCredentials fixes the standard output contract
// scripts/browse.sh reads: exactly two lines, the email before the password.
// Diagnostics stay off both streams on success.
//
// [Ja] TestRunDevCredentials_WritesTheCredentials は、scripts/browse.sh が読む標準出力の
// 契約を固定します。メールアドレス、パスワードの順で正確に 2 行とし、成功時はどちらの
// 出力にも診断情報を混ぜません。
func TestRunDevCredentials_WritesTheCredentials(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	var lookedUpRole string
	code := runDevCredentials(t.Context(), []string{"starter"}, "dev", func(role string) (*seed.Credentials, error) {
		lookedUpRole = role

		return stubCredentials, nil
	}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("runDevCredentials() exit code = %d, want 0", code)
	}
	if lookedUpRole != "starter" {
		t.Errorf("runDevCredentials() looked up %q, want %q", lookedUpRole, "starter")
	}
	if want := "starter@example.com\nseed-password\n"; stdout.String() != want {
		t.Errorf("runDevCredentials() stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("runDevCredentials() stderr = %q, want it to be empty", stderr.String())
	}
}

// TestRunDevCredentials_ReportsALookupFailure covers the failure the command
// meets most often: the roster is missing, the role is misspelled, or the file
// does not pass its checks.
//
// The standard output contract holds here too. A shell reads that stream as the
// credentials themselves, so a run that has none to give has to leave it empty
// and report through the logger, which writes to standard error.
//
// [Ja] TestRunDevCredentials_ReportsALookupFailure は、本コマンドがもっとも多く出会う
// 失敗を扱います。名簿が無い、役割の綴りを間違えた、ファイルが検査を通らない、といった
// 場合です。
//
// 標準出力の契約はここでも変わりません。シェルはそのストリームを資格情報そのものとして
// 読むため、渡せるものが無い実行はそれを空のままにし、標準エラー出力へ書くロガーで報告
// します。
func TestRunDevCredentials_ReportsALookupFailure(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := runDevCredentials(t.Context(), []string{"startr"}, "dev", func(string) (*seed.Credentials, error) {
		return nil, errors.New("the roster holds no account with the role \"startr\"")
	}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("runDevCredentials() exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("runDevCredentials() stdout = %q, want it to be empty", stdout.String())
	}
}

// TestDevCredentials_RejectsANonDevelopmentEnvironment checks the guard that
// stands in front of the lookup. The command prints a password, so it has to
// refuse anywhere the roster is not a development file, and it has to refuse
// before reading anything.
//
// [Ja] TestDevCredentials_RejectsANonDevelopmentEnvironment は、引きの手前に立つガードを
// 確認します。本コマンドはパスワードを出力するため、名簿が開発用のファイルでない場所では
// 実行を拒否する必要があり、しかも何かを読む前に拒否する必要があります。
func TestDevCredentials_RejectsANonDevelopmentEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appEnv  string
		wantErr bool
	}{
		{name: "development", appEnv: "dev"},
		{name: "test", appEnv: "test", wantErr: true},
		{name: "production", appEnv: "prod", wantErr: true},
		{name: "unset", appEnv: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lookedUp := false
			credentials, err := devCredentials(tt.appEnv, "starter", func(string) (*seed.Credentials, error) {
				lookedUp = true

				return stubCredentials, nil
			})

			if tt.wantErr {
				if err == nil {
					t.Fatal("devCredentials() should fail outside a development environment, but it succeeded")
				}
				if !strings.Contains(err.Error(), "development environment") {
					t.Errorf("devCredentials() error = %q, want the refusal of the environment", err)
				}
				if lookedUp {
					t.Error("devCredentials() looked the credentials up, want it to refuse the environment first")
				}

				return
			}

			if err != nil {
				t.Fatalf("devCredentials() error = %v", err)
			}
			if !lookedUp || credentials != stubCredentials {
				t.Errorf("devCredentials() = %+v, want the result of the lookup", credentials)
			}
		})
	}
}
