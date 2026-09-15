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

// stubCredentialsは、これらのテストで成功する引きが返す値です。
var stubCredentials = &seed.Credentials{Email: "starter@example.com", Password: "seed-password"}

// TestRun_DispatchesDevCredentialsUsageは、トップレベルコマンドがサブコマンド名を
// 取り除き、残りの引数をdevcredsへ渡すことを検証します。役割を指定しない実行は名簿を
// 読む前に応答されるため、この配線はディスク上の名簿無しで観測できます。
func TestRun_DispatchesDevCredentialsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"devcreds"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run()の終了コード = %d、期待値 = %d", code, exitUsage)
	}
	if want := "usage: groobb devcreds starter|replier|admin\n"; stderr.String() != want {
		t.Errorf("run()の標準エラー出力 = %q、期待値 = %q", stderr.String(), want)
	}
}

// TestRun_DispatchesDevCredentialsOutputは、トップレベルの振り分けから呼び出し側が
// 渡した標準出力までの成功経路を検証します。一時作業ディレクトリの名簿を使うことで、
// 開発者の追跡対象外の名簿とその資格情報をテストの依存にしません。
//
// 本テストは意図的に逐次実行します。t.Chdirとt.Setenvはどちらもプロセス全体の状態を
// 変更するため、Goもt.Parallelの後での利用を禁止しています。実行中は本パッケージの
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
role = "admin"
atname = "seeduser4"
email = "admin@example.com"
note = "administers the community"

[[users]]
role = "withdrawn"
atname = "seeduser3"
email = "withdrawn@example.com"
note = "leaves authorless posts"
`
	if err := os.WriteFile("seed-users.toml", []byte(roster), 0o600); err != nil {
		t.Fatalf("隔離した名簿の書き込みに失敗: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"devcreds", "starter"}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("run()の終了コード = %d、期待値 = 0", code)
	}
	if want := "starter@example.com\nseed-password\n"; stdout.String() != want {
		t.Errorf("run()の標準出力 = %q、期待値 = %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("run()の標準エラー出力 = %q、空を期待", stderr.String())
	}
}

// TestRunDevCredentials_RejectsAWrongArgumentCountは、役割を指定していない実行と、
// 2つ以上指定した実行を扱います。シェルは本コマンドの標準出力を資格情報そのものとして
// 読むため、答えられない実行はその出力を空のままにし、何が起きたのかは標準エラー出力で
// 告げる必要があります。
func TestRunDevCredentials_RejectsAWrongArgumentCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "役割の指定が無い", args: []string{}},
		{name: "役割を2つ以上指定", args: []string{"starter", "replier"}},
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
				t.Errorf("runDevCredentials()の終了コード = %d、期待値 = %d", code, exitUsage)
			}
			if stdout.Len() != 0 {
				t.Errorf("runDevCredentials()の標準出力 = %q、空を期待", stdout.String())
			}
			if want := "usage: groobb devcreds starter|replier|admin"; !strings.Contains(stderr.String(), want) {
				t.Errorf("runDevCredentials()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
			}

			// 引きが尋ねられるのは役割であるため、役割を指定していない実行と2つ
			// 指定した実行には、引くものがありません。引かないことを確認することが、引数の
			// 検査が名簿の手前に立っていることを言う方法になります。そうしなければ、その
			// 答えは、コマンドを開始した場所からそのファイルが読めるかどうかに委ねられます。
			if lookedUp {
				t.Error("runDevCredentials()が資格情報を引いた。引数の数への応答が先に行われることを期待")
			}
		})
	}
}

// TestRunDevCredentials_WritesTheCredentialsは、scripts/browse.shが読む標準出力の
// 契約を固定します。メールアドレス、パスワードの順で正確に2行とし、成功時はどちらの
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
		t.Errorf("runDevCredentials()の終了コード = %d、期待値 = 0", code)
	}
	if lookedUpRole != "starter" {
		t.Errorf("runDevCredentials()が引いた役割 = %q、期待値 = %q", lookedUpRole, "starter")
	}
	if want := "starter@example.com\nseed-password\n"; stdout.String() != want {
		t.Errorf("runDevCredentials()の標準出力 = %q、期待値 = %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("runDevCredentials()の標準エラー出力 = %q、空を期待", stderr.String())
	}
}

// TestRunDevCredentials_ReportsALookupFailureは、本コマンドがもっとも多く出会う
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
		t.Errorf("runDevCredentials()の終了コード = %d、期待値 = 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("runDevCredentials()の標準出力 = %q、空を期待", stdout.String())
	}
}

// TestDevCredentials_RejectsANonDevelopmentEnvironmentは、引きの手前に立つガードを
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
		{name: "未設定", appEnv: "", wantErr: true},
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
					t.Fatal("devCredentials()は開発環境の外では失敗するはずだが、成功した")
				}
				if !strings.Contains(err.Error(), "development environment") {
					t.Errorf("devCredentials()のエラー = %q、環境の拒否を期待", err)
				}
				if lookedUp {
					t.Error("devCredentials()が資格情報を引いた。環境の拒否が先に行われることを期待")
				}

				return
			}

			if err != nil {
				t.Fatalf("devCredentials()のエラー = %v", err)
			}
			if !lookedUp || credentials != stubCredentials {
				t.Errorf("devCredentials() = %+v、引いた結果を期待", credentials)
			}
		})
	}
}
