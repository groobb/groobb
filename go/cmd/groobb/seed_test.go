package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/seed"
)

// TestRun_DispatchesSeedは、トップレベルコマンドがseedサブコマンドとその引数を
// runSeedへ渡すことを検証します。
func TestRun_DispatchesSeed(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"seed", "--help"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run()の終了コード = %d、期待値 = %d", code, exitUsage)
	}
	if want := "usage: groobb seed [mature|cold-start]\n"; stderr.String() != want {
		t.Errorf("run()の標準エラー出力 = %q、期待値 = %q", stderr.String(), want)
	}
}

// TestRunSeed_RejectsAnArgumentThatNamesNoProfileは、プロファイルへ解決できない
// コマンドラインが、usageと使用方法の誤りを示す終了コードで応答されることを検証します。
// 実行はデータベースを空にするため、打ち間違えたフラグも綴りを誤ったプロファイルも、
// そこへ到達してはならないためです。
func TestRunSeed_RejectsAnArgumentThatNamesNoProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "フラグ", args: []string{"--help"}},
		{name: "未知のプロファイル", args: []string{"coldstart"}},
		{name: "引数が2つ以上", args: []string{"mature", "cold-start"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			code := runSeed(context.Background(), tt.args, &stderr)

			if code != exitUsage {
				t.Errorf("runSeed(%q)の終了コード = %d、期待値 = %d", tt.args, code, exitUsage)
			}
			if want := "usage: groobb seed"; !strings.Contains(stderr.String(), want) {
				t.Errorf("runSeed(%q)の標準エラー出力 = %q、%q を含むことを期待", tt.args, stderr.String(), want)
			}
		})
	}
}

// TestSeedProfileは、コマンドラインがどのコミュニティの状態へ解決するのかを検証
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
		{name: "引数が無い", args: nil, want: "mature"},
		{name: "成熟したコミュニティ", args: []string{"mature"}, want: "mature"},
		{name: "最初の日", args: []string{"cold-start"}, want: "cold-start"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			profile, ok := seedProfile(tt.args)

			if !ok {
				t.Fatalf("seedProfile(%q)がプロファイルを返さなかった、期待値 = %q", tt.args, tt.want)
			}
			if profile.Name() != tt.want {
				t.Errorf("seedProfile(%q) = %q、期待値 = %q", tt.args, profile.Name(), tt.want)
			}
		})
	}
}

// ガードが読むのが生のAPP_ENVであることを検証します。config.Loadは未設定の
// APP_ENVを開発環境として読むため、読み込み済みの設定に対する検査では、APP_ENVを設定
// しない本番プロセスを通してしまいます。環境からは読み込みに必要な設定を外してあり、
// それが、拒否が読み込みより前に起きたことを示します。
func TestSeedDatabase_RejectsANonDevelopmentEnvironment(t *testing.T) {
	t.Setenv("GROOBB_PORT", "")
	t.Setenv("GROOBB_DATABASE_PATH", "")
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "")

	err := seedDatabase(context.Background(), "prod", seed.DefaultProfile())
	if err == nil {
		t.Fatal("seedDatabase()は開発環境の外では失敗するはずだが、成功した")
	}
	if !strings.Contains(err.Error(), "development environment") {
		t.Errorf("seedDatabase()のエラー = %q、環境の拒否を期待", err)
	}
}
