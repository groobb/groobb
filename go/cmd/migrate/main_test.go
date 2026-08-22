package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
)

// setMigrationCommandEnv sets the environment required by the migration
// command and returns the throwaway database path.
//
// [Ja] setMigrationCommandEnv はマイグレーションコマンドに必要な環境変数を設定し、
// 使い捨てのデータベースパスを返します。
func setMigrationCommandEnv(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "groobb.sqlite")
	t.Setenv("APP_ENV", "test")
	t.Setenv("GROOBB_PORT", "8080")
	t.Setenv("GROOBB_DATABASE_PATH", path)
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "groobb-test-continuation-token-key-32-bytes")
	t.Setenv("GROOBB_EMAIL_PROVIDER", "")

	return path
}

// assertTableState verifies whether the named table exists in the database at
// path.
//
// [Ja] assertTableState は、path のデータベースに指定した名前のテーブルが存在するかを
// 検証します。
func assertTableState(t *testing.T, path, table string, wantExists bool) {
	t.Helper()

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to open the database for verification: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close the verification database: %v", err)
		}
	}()

	var name string
	err = db.Reader.QueryRowContext(
		context.Background(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&name)

	if wantExists {
		if err != nil {
			t.Fatalf("the %s table should exist, but querying it failed: %v", table, err)
		}
		return
	}

	if err == nil {
		t.Fatalf("the %s table should not exist, but it does", table)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("querying the removed %s table returned an unexpected error: %v", table, err)
	}
}

// TestRun_RejectsInvalidArguments verifies that the command requires exactly
// one known subcommand.
//
// [Ja] TestRun_RejectsInvalidArguments は、コマンドが既知のサブコマンドをちょうど
// 1 つ要求することを検証します。
func TestRun_RejectsInvalidArguments(t *testing.T) {
	setMigrationCommandEnv(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing subcommand", wantErr: "usage: migrate up|down"},
		{name: "too many arguments", args: []string{"up", "down"}, wantErr: "usage: migrate up|down"},
		{
			name:    "unknown subcommand",
			args:    []string{"status"},
			wantErr: "unknown subcommand \"status\": use \"up\" or \"down\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args)
			if err == nil {
				t.Fatal("run() should fail, but it succeeded")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("run() error = %q, want %q", err, tt.wantErr)
			}
		})
	}
}

// TestRun_ReturnsConfigurationErrors verifies that configuration failures are
// returned instead of being hidden.
//
// [Ja] TestRun_ReturnsConfigurationErrors は、設定の失敗が隠されず返されることを
// 検証します。
func TestRun_ReturnsConfigurationErrors(t *testing.T) {
	setMigrationCommandEnv(t)
	t.Setenv("GROOBB_PORT", "")

	err := run([]string{"up"})
	if err == nil {
		t.Fatal("run() should fail when required configuration is missing, but it succeeded")
	}
	if !strings.Contains(err.Error(), "failed to load the configuration") {
		t.Errorf("run() error = %q, want a configuration error", err)
	}
}

// TestRun_ReturnsDatabaseOpenErrors verifies that connection failures are
// returned instead of being hidden.
//
// [Ja] TestRun_ReturnsDatabaseOpenErrors は、データベース接続の失敗が隠されず返される
// ことを検証します。
func TestRun_ReturnsDatabaseOpenErrors(t *testing.T) {
	setMigrationCommandEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", filepath.Join(t.TempDir(), "missing", "groobb.sqlite"))

	err := run([]string{"up"})
	if err == nil {
		t.Fatal("run() should fail when the database cannot be opened, but it succeeded")
	}
	if !strings.Contains(err.Error(), "failed to open the database") {
		t.Errorf("run() error = %q, want a database open error", err)
	}
}

// TestRun_AppliesAndRollsBackMigrations verifies that the up and down
// subcommands dispatch through the full embedded-migration path. up applies
// every migration, so it is checked against a table from the first one; down
// reverts only the most recent, so it is checked against a table from that one.
//
// [Ja] TestRun_AppliesAndRollsBackMigrations は、up と down の各サブコマンドが
// 埋め込みマイグレーションの経路全体へ正しく振り分けられることを検証します。up は
// すべてのマイグレーションを適用するため最初のマイグレーションのテーブルで、down は
// 最新の 1 本だけを取り消すためそのマイグレーションのテーブルで確認します。
func TestRun_AppliesAndRollsBackMigrations(t *testing.T) {
	path := setMigrationCommandEnv(t)

	if err := run([]string{"up"}); err != nil {
		t.Fatalf("run(up) returned an unexpected error: %v", err)
	}
	assertTableState(t, path, "users", true)
	assertTableState(t, path, "river_job", true)

	if err := run([]string{"down"}); err != nil {
		t.Fatalf("run(down) returned an unexpected error: %v", err)
	}
	assertTableState(t, path, "river_job", false)
}
