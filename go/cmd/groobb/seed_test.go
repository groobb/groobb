package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/seed"
)

// TestRun_DispatchesSeed verifies that the top-level command sends the seed
// subcommand and its arguments to runSeed.
//
// [Ja] TestRun_DispatchesSeed は、トップレベルコマンドが seed サブコマンドとその引数を
// runSeed へ渡すことを検証します。
func TestRun_DispatchesSeed(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"seed", "--help"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run() exit code = %d, want %d", code, exitUsage)
	}
	if want := "usage: groobb seed [mature|cold-start]\n"; stderr.String() != want {
		t.Errorf("run() stderr = %q, want %q", stderr.String(), want)
	}
}

// TestRunSeed_RejectsAnArgumentThatNamesNoProfile verifies that a command line
// the subcommand cannot resolve to a profile is answered with the usage and the
// usage exit code. A run empties the database, so neither a mistyped flag nor a
// misspelled profile may reach one.
//
// [Ja] TestRunSeed_RejectsAnArgumentThatNamesNoProfile は、プロファイルへ解決できない
// コマンドラインが、usage と使用方法の誤りを示す終了コードで応答されることを検証します。
// 実行はデータベースを空にするため、打ち間違えたフラグも綴りを誤ったプロファイルも、
// そこへ到達してはならないためです。
func TestRunSeed_RejectsAnArgumentThatNamesNoProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "a flag", args: []string{"--help"}},
		{name: "an unknown profile", args: []string{"coldstart"}},
		{name: "more than one argument", args: []string{"mature", "cold-start"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			code := runSeed(context.Background(), tt.args, &stderr)

			if code != exitUsage {
				t.Errorf("runSeed(%q) exit code = %d, want %d", tt.args, code, exitUsage)
			}
			if want := "usage: groobb seed"; !strings.Contains(stderr.String(), want) {
				t.Errorf("runSeed(%q) stderr = %q, want it to contain %q", tt.args, stderr.String(), want)
			}
		})
	}
}

// TestSeedProfile verifies which community state a command line resolves to: the
// mature one when it names none, and the one it names otherwise.
//
// The resolution is checked here rather than by running the subcommand, because
// a run that reached the database would empty whichever one the environment
// points at.
//
// [Ja] TestSeedProfile は、コマンドラインがどのコミュニティの状態へ解決するのかを検証
// します。何も指定しなければ成熟した状態、指定すればその状態です。
//
// 解決をサブコマンドの実行ではなくここで確かめるのは、データベースへ到達した実行が、
// 環境の指す先を空にしてしまうためです。
func TestSeedProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no argument", args: nil, want: "mature"},
		{name: "the mature community", args: []string{"mature"}, want: "mature"},
		{name: "the first day", args: []string{"cold-start"}, want: "cold-start"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			profile, ok := seedProfile(tt.args)

			if !ok {
				t.Fatalf("seedProfile(%q) reported no profile, want %q", tt.args, tt.want)
			}
			if profile.Name() != tt.want {
				t.Errorf("seedProfile(%q) = %q, want %q", tt.args, profile.Name(), tt.want)
			}
		})
	}
}

// TestSeedDatabase_RejectsANonDevelopmentEnvironment verifies that the raw
// APP_ENV is what the guard reads. config.Load reads an unset APP_ENV as
// development, so a check made on the loaded configuration would let a
// production process that never sets it through. The environment is left
// without the settings a load needs, which is what says the refusal came before
// the load.
//
// [Ja] ガードが読むのが生の APP_ENV であることを検証します。config.Load は未設定の
// APP_ENV を開発環境として読むため、読み込み済みの設定に対する検査では、APP_ENV を設定
// しない本番プロセスを通してしまいます。環境からは読み込みに必要な設定を外してあり、
// それが、拒否が読み込みより前に起きたことを示します。
func TestSeedDatabase_RejectsANonDevelopmentEnvironment(t *testing.T) {
	t.Setenv("GROOBB_PORT", "")
	t.Setenv("GROOBB_DATABASE_PATH", "")
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "")

	err := seedDatabase(context.Background(), "prod", seed.DefaultProfile())
	if err == nil {
		t.Fatal("seedDatabase() should fail outside a development environment, but it succeeded")
	}
	if !strings.Contains(err.Error(), "development environment") {
		t.Errorf("seedDatabase() error = %q, want the refusal of the environment", err)
	}
}
