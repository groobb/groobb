package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestRun_RejectsAnInvocationWithoutAKnownSubcommandは、サブコマンドを
// 指定していない / 本コマンドの知らないサブコマンドを指定したコマンドラインがusageと
// 使用方法の誤りを示す終了コードで応答されること、そして未知の名前が引用符付きで
// 出力され、打ち間違いが見えることを検証します。
//
// serveサブコマンドはここでは扱いません。ポートを占有しシャットダウンまでブロック
// するため、その確認は振り分けのテストではなくサーバーの実行の担当です。
func TestRun_RejectsAnInvocationWithoutAKnownSubcommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name: "サブコマンドの指定が無い",
			wantContains: []string{
				"usage: groobb <command>",
				"serve",
				"  migrate up       apply the pending migrations",
				"  migrate down     roll back the most recent migration",
				"  seed [profile]   rebuild the development database with seed data",
				"  devcreds <role>  print the sign-in credentials of a seeded account",
			},
		},
		{
			name:         "未知のサブコマンド",
			args:         []string{"nosuchcommand"},
			wantContains: []string{`unknown subcommand: "nosuchcommand"`, "usage: groobb <command>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			code := run(tt.args, io.Discard, &stderr)

			if code != exitUsage {
				t.Errorf("run()の終了コード = %d、期待値 = %d", code, exitUsage)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("run()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
				}
			}
		})
	}
}
