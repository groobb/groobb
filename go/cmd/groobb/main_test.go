package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestRun_RejectsAnInvocationWithoutAKnownSubcommand verifies that a command
// line naming no subcommand, or one this command does not know, is answered
// with the usage and the usage exit code, and that an unknown name is quoted
// back so a typo is visible in the output.
//
// The serve subcommand is not covered here: it binds a port and blocks until
// shutdown, so exercising it belongs to a run of the server rather than to a
// test of the dispatch.
//
// [Ja] TestRun_RejectsAnInvocationWithoutAKnownSubcommand は、サブコマンドを
// 指定していない / 本コマンドの知らないサブコマンドを指定したコマンドラインが usage と
// 使用方法の誤りを示す終了コードで応答されること、そして未知の名前が引用符付きで
// 出力され、打ち間違いが見えることを検証します。
//
// serve サブコマンドはここでは扱いません。ポートを占有しシャットダウンまでブロック
// するため、その確認は振り分けのテストではなくサーバーの実行の担当です。
func TestRun_RejectsAnInvocationWithoutAKnownSubcommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name: "missing subcommand",
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
			name:         "unknown subcommand",
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
				t.Errorf("run() exit code = %d, want %d", code, exitUsage)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("run() stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
		})
	}
}
