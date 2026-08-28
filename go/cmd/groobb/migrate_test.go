package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
)

// setMigrationEnv sets the environment the migrate subcommand needs and returns
// the throwaway database path.
//
// [Ja] setMigrationEnv は migrate サブコマンドが必要とする環境変数を設定し、
// 使い捨てのデータベースパスを返します。
func setMigrationEnv(t *testing.T) string {
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

// countAppliedMigrations returns how many migrations the database at path
// records as applied.
//
// It reads the version table goose keeps instead of looking for a table that one
// particular migration creates, so that what it reports keeps its meaning as
// migrations are added.
//
// [Ja] countAppliedMigrations は、path のデータベースが適用済みとして記録している
// マイグレーションの本数を返します。
//
// 特定のマイグレーションが作るテーブルを探すのではなく goose が持つバージョン管理
// テーブルを読むのは、マイグレーションが増えても報告する内容が意味を保つようにするため
// です。
func countAppliedMigrations(t *testing.T, path string) int {
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

	var count int
	err = db.Reader.QueryRowContext(
		context.Background(),
		"SELECT count(*) FROM goose_db_version WHERE is_applied",
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count the applied migrations: %v", err)
	}

	return count
}

// TestRunMigrate_RejectsInvalidArguments verifies that the subcommand requires
// exactly one known direction, answers anything else with the usage and the
// usage exit code, and quotes an unknown direction back so a typo is visible in
// the output.
//
// [Ja] TestRunMigrate_RejectsInvalidArguments は、本サブコマンドが既知の方向を
// ちょうど 1 つ要求し、それ以外を usage と使用方法の誤りを示す終了コードで応答すること、
// そして未知の方向が引用符付きで出力され、打ち間違いが見えることを検証します。
func TestRunMigrate_RejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name:         "missing direction",
			wantContains: []string{"usage: groobb migrate up|down"},
		},
		{
			name:         "too many arguments",
			args:         []string{"up", "down"},
			wantContains: []string{"usage: groobb migrate up|down"},
		},
		{
			name:         "unknown direction",
			args:         []string{"status"},
			wantContains: []string{`unknown migration direction: "status"`, "usage: groobb migrate up|down"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			code := runMigrate(context.Background(), tt.args, &stderr)

			if code != exitUsage {
				t.Errorf("runMigrate() exit code = %d, want %d", code, exitUsage)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("runMigrate() stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
		})
	}
}

// TestRunMigrate_ReturnsFailureExitCode verifies that a failure of the requested
// migration work is distinguished from a usage error by exit code 1.
//
// [Ja] TestRunMigrate_ReturnsFailureExitCode は、依頼されたマイグレーション処理の失敗が
// 使用方法の誤りとは区別され、終了コード 1 になることを検証します。
func TestRunMigrate_ReturnsFailureExitCode(t *testing.T) {
	setMigrationEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", filepath.Join(t.TempDir(), "missing", "groobb.sqlite"))

	var stderr bytes.Buffer

	code := runMigrate(context.Background(), []string{"up"}, &stderr)

	if code != 1 {
		t.Errorf("runMigrate() exit code = %d, want 1", code)
	}
}

// TestMigrateDatabase_ReturnsConfigurationErrors verifies that configuration
// failures are returned instead of being hidden.
//
// [Ja] TestMigrateDatabase_ReturnsConfigurationErrors は、設定の失敗が隠されず
// 返されることを検証します。
func TestMigrateDatabase_ReturnsConfigurationErrors(t *testing.T) {
	setMigrationEnv(t)
	t.Setenv("GROOBB_PORT", "")

	err := migrateDatabase(context.Background(), database.Migrate)
	if err == nil {
		t.Fatal("migrateDatabase() should fail when required configuration is missing, but it succeeded")
	}
	if !strings.Contains(err.Error(), "failed to load the configuration") {
		t.Errorf("migrateDatabase() error = %q, want a configuration error", err)
	}
}

// TestMigrateDatabase_ReturnsDatabaseOpenErrors verifies that connection
// failures are returned instead of being hidden.
//
// [Ja] TestMigrateDatabase_ReturnsDatabaseOpenErrors は、データベース接続の失敗が
// 隠されず返されることを検証します。
func TestMigrateDatabase_ReturnsDatabaseOpenErrors(t *testing.T) {
	setMigrationEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", filepath.Join(t.TempDir(), "missing", "groobb.sqlite"))

	err := migrateDatabase(context.Background(), database.Migrate)
	if err == nil {
		t.Fatal("migrateDatabase() should fail when the database cannot be opened, but it succeeded")
	}
	if !strings.Contains(err.Error(), "failed to open the database") {
		t.Errorf("migrateDatabase() error = %q, want a database open error", err)
	}
}

// TestRunMigrate_AppliesAndRollsBackMigrations verifies that the up and down
// directions dispatch through the full embedded-migration path. up applies every
// migration, so it is checked against tables from the first and the last one.
//
// down reverts only the most recent, which is checked as one fewer migration
// recorded as applied while the first migration's table is still there. Checking
// instead that a named table has disappeared would say the same thing only for
// as long as the migration creating it stays the newest one.
//
// [Ja] TestRunMigrate_AppliesAndRollsBackMigrations は、up と down の各方向が
// 埋め込みマイグレーションの経路全体へ正しく振り分けられることを検証します。up は
// すべてのマイグレーションを適用するため、最初と最後のマイグレーションのテーブルで
// 確認します。
//
// down は最新の 1 本だけを取り消すため、適用済みとして記録されている本数が 1 つ減り、
// かつ最初のマイグレーションのテーブルが残っていることで確認します。名指ししたテーブルが
// 消えたことで確認すると、それを作るマイグレーションが最新であり続ける間しか同じことを
// 意味しません。
func TestRunMigrate_AppliesAndRollsBackMigrations(t *testing.T) {
	path := setMigrationEnv(t)

	var stderr bytes.Buffer

	if code := runMigrate(context.Background(), []string{"up"}, &stderr); code != 0 {
		t.Fatalf("runMigrate(up) exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	assertTableState(t, path, "users", true)
	assertTableState(t, path, "river_job", true)
	appliedAfterUp := countAppliedMigrations(t, path)

	if code := runMigrate(context.Background(), []string{"down"}, &stderr); code != 0 {
		t.Fatalf("runMigrate(down) exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	if applied := countAppliedMigrations(t, path); applied != appliedAfterUp-1 {
		t.Errorf("applied migrations after down = %d, want %d", applied, appliedAfterUp-1)
	}
	assertTableState(t, path, "users", true)
}

// TestRun_DispatchesMigrate verifies that the top-level dispatch reaches the
// migrate subcommand with the arguments that follow its name, and with the
// stream its caller passed. An unknown direction is used because it is answered
// before the configuration is read, so the wiring is observable without a
// database: the quoted name in the output is what an invocation carrying the
// subcommand name along with it could not produce.
//
// [Ja] TestRun_DispatchesMigrate は、トップレベルの振り分けが、migrate サブコマンドへ
// その名前に続く引数と、呼び出し側が渡したストリームを伴って到達することを検証します。
// 未知の方向を使うのは、それが設定の読み込みより前に応答されるためで、データベース
// 無しで配線を観測できます。出力に現れる引用符付きの名前は、サブコマンド名を一緒に
// 引き渡してしまう実行では作れないものです。
func TestRun_DispatchesMigrate(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"migrate", "status"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run() exit code = %d, want %d", code, exitUsage)
	}
	for _, want := range []string{`unknown migration direction: "status"`, "usage: groobb migrate up|down"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("run() stderr = %q, want it to contain %q", stderr.String(), want)
		}
	}
}
