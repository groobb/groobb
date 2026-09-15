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

// setDatabaseEnvは、設定されたデータベースを開くサブコマンドが必要とする環境変数を
// 設定し、使い捨てのデータベースファイルのパスを返します。
//
// このパスが指すのはまだ存在しないファイルで、migrateサブコマンドはそこから始めます。
// スキーマが入った状態のデータベースを必要とするテストは、その後で設定を自前のものへ
// 向け直します。
func setDatabaseEnv(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "groobb.sqlite")
	t.Setenv("APP_ENV", "test")
	t.Setenv("GROOBB_PORT", "8080")
	t.Setenv("GROOBB_DATABASE_PATH", path)
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "groobb-test-continuation-token-key-32-bytes")
	t.Setenv("GROOBB_EMAIL_PROVIDER", "")

	return path
}

// assertTableStateは、pathのデータベースに指定した名前のテーブルが存在するかを
// 検証します。
func assertTableState(t *testing.T, path, table string, wantExists bool) {
	t.Helper()

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("検証用のデータベースのオープンに失敗: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("検証用のデータベースのクローズに失敗: %v", err)
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
			t.Fatalf("%s テーブルが存在するはずだが、問い合わせに失敗: %v", table, err)
		}
		return
	}

	if err == nil {
		t.Fatalf("%s テーブルは存在しないはずだが、存在する", table)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("削除した %s テーブルへの問い合わせが予期しないエラーを返した: %v", table, err)
	}
}

// countAppliedMigrationsは、pathのデータベースが適用済みとして記録している
// マイグレーションの本数を返します。
//
// 特定のマイグレーションが作るテーブルを探すのではなくgooseが持つバージョン管理
// テーブルを読むのは、マイグレーションが増えても報告する内容が意味を保つようにするため
// です。
func countAppliedMigrations(t *testing.T, path string) int {
	t.Helper()

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("検証用のデータベースのオープンに失敗: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("検証用のデータベースのクローズに失敗: %v", err)
		}
	}()

	var count int
	err = db.Reader.QueryRowContext(
		context.Background(),
		"SELECT count(*) FROM goose_db_version WHERE is_applied",
	).Scan(&count)
	if err != nil {
		t.Fatalf("適用済みマイグレーションの計数に失敗: %v", err)
	}

	return count
}

// TestRunMigrate_RejectsInvalidArgumentsは、本サブコマンドが既知の方向を
// ちょうど1つ要求し、それ以外をusageと使用方法の誤りを示す終了コードで応答すること、
// そして未知の方向が引用符付きで出力され、打ち間違いが見えることを検証します。
func TestRunMigrate_RejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name:         "方向の指定が無い",
			wantContains: []string{"usage: groobb migrate up|down"},
		},
		{
			name:         "引数が多すぎる",
			args:         []string{"up", "down"},
			wantContains: []string{"usage: groobb migrate up|down"},
		},
		{
			name:         "未知の方向",
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
				t.Errorf("runMigrate()の終了コード = %d、期待値 = %d", code, exitUsage)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("runMigrate()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
				}
			}
		})
	}
}

// TestRunMigrate_ReturnsFailureExitCodeは、依頼されたマイグレーション処理の失敗が
// 使用方法の誤りとは区別され、終了コード1になることを検証します。
func TestRunMigrate_ReturnsFailureExitCode(t *testing.T) {
	setDatabaseEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", filepath.Join(t.TempDir(), "missing", "groobb.sqlite"))

	var stderr bytes.Buffer

	code := runMigrate(context.Background(), []string{"up"}, &stderr)

	if code != 1 {
		t.Errorf("runMigrate()の終了コード = %d、期待値 = 1", code)
	}
}

// TestMigrateDatabase_ReturnsConfigurationErrorsは、設定の失敗が隠されず
// 返されることを検証します。
func TestMigrateDatabase_ReturnsConfigurationErrors(t *testing.T) {
	setDatabaseEnv(t)
	t.Setenv("GROOBB_PORT", "")

	err := migrateDatabase(context.Background(), database.Migrate)
	if err == nil {
		t.Fatal("必須の設定が欠けているときmigrateDatabase()は失敗するはずだが、成功した")
	}
	if !strings.Contains(err.Error(), "failed to load the configuration") {
		t.Errorf("migrateDatabase()のエラー = %q、設定のエラーを期待", err)
	}
}

// TestMigrateDatabase_ReturnsDatabaseOpenErrorsは、データベース接続の失敗が
// 隠されず返されることを検証します。
func TestMigrateDatabase_ReturnsDatabaseOpenErrors(t *testing.T) {
	setDatabaseEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", filepath.Join(t.TempDir(), "missing", "groobb.sqlite"))

	err := migrateDatabase(context.Background(), database.Migrate)
	if err == nil {
		t.Fatal("データベースを開けないときmigrateDatabase()は失敗するはずだが、成功した")
	}
	if !strings.Contains(err.Error(), "failed to open the database") {
		t.Errorf("migrateDatabase()のエラー = %q、データベースのオープンのエラーを期待", err)
	}
}

// TestRunMigrate_AppliesAndRollsBackMigrationsは、upとdownの各方向が
// 埋め込みマイグレーションの経路全体へ正しく振り分けられることを検証します。upは
// すべてのマイグレーションを適用するため、最初と最後のマイグレーションのテーブルで
// 確認します。
//
// downは最新の1本だけを取り消すため、適用済みとして記録されている本数が1つ減り、
// かつ最初のマイグレーションのテーブルが残っていることで確認します。名指ししたテーブルが
// 消えたことで確認すると、それを作るマイグレーションが最新であり続ける間しか同じことを
// 意味しません。
func TestRunMigrate_AppliesAndRollsBackMigrations(t *testing.T) {
	path := setDatabaseEnv(t)

	var stderr bytes.Buffer

	if code := runMigrate(context.Background(), []string{"up"}, &stderr); code != 0 {
		t.Fatalf("runMigrate(up)の終了コード = %d、期待値 = 0 (標準エラー出力: %q)", code, stderr.String())
	}
	assertTableState(t, path, "users", true)
	assertTableState(t, path, "river_job", true)
	appliedAfterUp := countAppliedMigrations(t, path)

	if code := runMigrate(context.Background(), []string{"down"}, &stderr); code != 0 {
		t.Fatalf("runMigrate(down)の終了コード = %d、期待値 = 0 (標準エラー出力: %q)", code, stderr.String())
	}
	if applied := countAppliedMigrations(t, path); applied != appliedAfterUp-1 {
		t.Errorf("down後の適用済みマイグレーションの本数 = %d、期待値 = %d", applied, appliedAfterUp-1)
	}
	assertTableState(t, path, "users", true)
}

// TestRun_DispatchesMigrateは、トップレベルの振り分けが、migrateサブコマンドへ
// その名前に続く引数と、呼び出し側が渡したストリームを伴って到達することを検証します。
// 未知の方向を使うのは、それが設定の読み込みより前に応答されるためで、データベース
// 無しで配線を観測できます。出力に現れる引用符付きの名前は、サブコマンド名を一緒に
// 引き渡してしまう実行では作れないものです。
func TestRun_DispatchesMigrate(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"migrate", "status"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run()の終了コード = %d、期待値 = %d", code, exitUsage)
	}
	for _, want := range []string{`unknown migration direction: "status"`, "usage: groobb migrate up|down"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("run()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
		}
	}
}
