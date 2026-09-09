package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// setUpRoleDatabase gives the test a migrated database, points the configuration
// the subcommand loads at it, and returns the pools the test reads it back
// through.
//
// The subcommand opens the file itself, so the test holds a second set of pools
// on the same database rather than the ones the run uses. That is what lets an
// assertion read the rows the subcommand wrote, from outside the process's own
// view of them.
//
// [Ja] setUpRoleDatabase はテストにマイグレーション済みのデータベースを与え、サブコマンドが
// 読み込む設定をそこへ向け、テストがそれを読み戻すためのプールを返します。
//
// サブコマンドはファイルを自分で開くため、テストが持つのは実行が使うプールではなく、同じ
// データベースに対する 2 組目のプールです。これによって、サブコマンドが書き込んだ行を、
// プロセス自身の見え方の外から検証できます。
func setUpRoleDatabase(t *testing.T) *database.DB {
	t.Helper()

	path := testutil.SetupDBPath(t)
	setDatabaseEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", path)

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to open the database for verification: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close the verification database: %v", err)
		}
	})

	return db
}

// newRoleLogger returns the logger a test hands the subcommand, so that the
// diagnostics an invocation reports land in w rather than in the test process's
// own standard error, where an assertion cannot reach them.
//
// [Ja] newRoleLogger は、テストがサブコマンドへ渡す logger を返します。実行が報告する
// 診断が、検証の手が届かないテストプロセス自身の標準エラーではなく、w に届くようにする
// ためです。
func newRoleLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil))
}

// assertHoldsAdmin verifies whether the user holds the built-in admin role.
//
// It reads through the repository the application resolves an actor's permission
// with, so what it checks is the assignment as the application sees it.
//
// [Ja] assertHoldsAdmin は、ユーザーが組み込みの admin ロールを持つかどうかを検証します。
//
// 読み取りは、アプリケーションが操作者の権限を解決するのに使うリポジトリを通します。検査の
// 対象を、アプリケーションから見える割当そのものにするためです。
func assertHoldsAdmin(t *testing.T, db *database.DB, userID model.UserID, wantHolds bool) {
	t.Helper()

	roles, err := repository.NewRoleRepository(db).ListByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUserID(%v) error = %v", userID, err)
	}

	holds := false
	for _, role := range roles {
		if role.Name == model.RoleNameAdmin {
			holds = true
		}
	}

	if holds != wantHolds {
		t.Errorf("the user holds the %s role = %v, want %v", model.RoleNameAdmin, holds, wantHolds)
	}
}

// TestRunRole_RejectsInvalidArguments verifies that the subcommand requires an
// action, an atname and a role, answers anything else with the usage and the
// usage exit code, and quotes an unknown action back so a typo is visible in the
// output.
//
// [Ja] TestRunRole_RejectsInvalidArguments は、本サブコマンドが動作・atname・ロールを
// 要求し、それ以外を usage と使用方法の誤りを示す終了コードで応答すること、そして未知の
// 動作が引用符付きで出力され、打ち間違いが見えることを検証します。
func TestRunRole_RejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name:         "no arguments",
			wantContains: []string{"usage: groobb role grant|revoke <atname> <role>"},
		},
		{
			name:         "missing role",
			args:         []string{"grant", "someone"},
			wantContains: []string{"usage: groobb role grant|revoke <atname> <role>"},
		},
		{
			name:         "too many arguments",
			args:         []string{"grant", "someone", "admin", "admin"},
			wantContains: []string{"usage: groobb role grant|revoke <atname> <role>"},
		},
		{
			name:         "unknown action",
			args:         []string{"give", "someone", "admin"},
			wantContains: []string{`unknown role action: "give"`, "usage: groobb role grant|revoke <atname> <role>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			code := runRole(context.Background(), tt.args, &stderr, newRoleLogger(&stderr))

			if code != exitUsage {
				t.Errorf("runRole() exit code = %d, want %d", code, exitUsage)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("runRole() stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
		})
	}
}

// TestRunRole_GrantsAndRevokesTheRole verifies the two directions against a real
// database: the account named by its atname ends up holding the role, and a
// second invocation takes it away again.
//
// Another administrator is created first, because the community keeps one at all
// times and the revoke would otherwise be refused for that reason rather than
// performed.
//
// [Ja] TestRunRole_GrantsAndRevokesTheRole は、実データベースに対して 2 つの方向を検証
// します。atname で名指ししたアカウントがロールを持つ状態になり、2 度目の実行がそれを再び
// 取り上げることです。
//
// 先に別の管理者を作るのは、コミュニティが常に管理者を 1 人以上持つためです。そうしなければ
// 剥奪は実行されず、その理由で拒否されることになります。
func TestRunRole_GrantsAndRevokesTheRole(t *testing.T) {
	db := setUpRoleDatabase(t)

	atname := "roleholder"
	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(testutil.NewUserBuilder(t, db).Build()).Build()

	var stderr bytes.Buffer

	if code := runRole(context.Background(), []string{"grant", atname, string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr)); code != 0 {
		t.Fatalf("runRole(grant) exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	assertHoldsAdmin(t, db, userID, true)

	if code := runRole(context.Background(), []string{"revoke", atname, string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr)); code != 0 {
		t.Fatalf("runRole(revoke) exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}
	assertHoldsAdmin(t, db, userID, false)
}

// TestRunRole_RefusesToRevokeTheLastAdministrator verifies that the protection
// the UseCase applies reaches the command line: the last holder keeps the role
// and the failure is told apart from a usage error by exit code 1.
//
// [Ja] TestRunRole_RefusesToRevokeTheLastAdministrator は、UseCase が課す保護がコマンド
// ラインまで届くことを検証します。最後の保持者はロールを持ったままになり、その失敗は終了
// コード 1 によって使用方法の誤りと区別されます。
func TestRunRole_RefusesToRevokeTheLastAdministrator(t *testing.T) {
	db := setUpRoleDatabase(t)

	atname := "lastadmin"
	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

	var stderr bytes.Buffer

	code := runRole(context.Background(), []string{"revoke", atname, string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr))

	if code != 1 {
		t.Errorf("runRole(revoke) exit code = %d, want 1", code)
	}
	if want := "最後の管理者からの剥奪"; !strings.Contains(stderr.String(), want) {
		t.Errorf("runRole(revoke) stderr = %q, want it to contain %q", stderr.String(), want)
	}
	assertHoldsAdmin(t, db, userID, true)
}

// TestRunRole_ReportsANameThatCarriesNothing verifies that an atname no account
// holds and a role name no role carries both fail the requested work rather than
// the command line: the exit code is 1, and the account the invocation named
// gains nothing.
//
// [Ja] TestRunRole_ReportsANameThatCarriesNothing は、どのアカウントも持たない atname と、
// どのロールも持たないロール名が、どちらもコマンドラインの誤りではなく依頼された処理の失敗に
// なることを検証します。終了コードは 1 であり、実行が名指ししたアカウントは何も得ません。
func TestRunRole_ReportsANameThatCarriesNothing(t *testing.T) {
	db := setUpRoleDatabase(t)

	atname := "presentuser"
	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()

	tests := []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{
			name:        "an atname no account holds",
			args:        []string{"grant", "absentuser", string(model.RoleNameAdmin)},
			wantMessage: `no user has the atname \"absentuser\"`,
		},
		{
			name:        "a role name no role carries",
			args:        []string{"grant", atname, "moderator"},
			wantMessage: "ロールが見つからない: role_name=moderator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer

			code := runRole(context.Background(), tt.args, &stderr, newRoleLogger(&stderr))

			if code != 1 {
				t.Errorf("runRole() exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), tt.wantMessage) {
				t.Errorf("runRole() stderr = %q, want it to contain %q", stderr.String(), tt.wantMessage)
			}
			assertHoldsAdmin(t, db, userID, false)
		})
	}
}

// TestRunRole_ReturnsFailureExitCode verifies that a failure to reach the
// database is told apart from a usage error by exit code 1, and that the
// diagnostic names what the subcommand was doing.
//
// This is the failure that does not arrive as a model.AppError, so it is the one
// that reaches the other branch of the report: the branch the tests above never
// take.
//
// [Ja] TestRunRole_ReturnsFailureExitCode は、データベースへ到達できない失敗が終了コード 1
// によって使用方法の誤りと区別されること、そして診断がサブコマンドの行っていたことを
// 名指しすることを検証します。
//
// これは model.AppError としては届かない失敗であり、報告のもう一方の分岐へ到達するのは
// これです。上のテストがどれも通らない分岐です。
func TestRunRole_ReturnsFailureExitCode(t *testing.T) {
	setDatabaseEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", filepath.Join(t.TempDir(), "missing", "groobb.sqlite"))

	var stderr bytes.Buffer

	code := runRole(context.Background(), []string{"grant", "someone", string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr))

	if code != 1 {
		t.Errorf("runRole() exit code = %d, want 1", code)
	}
	if want := "failed to change the role of the user"; !strings.Contains(stderr.String(), want) {
		t.Errorf("runRole() stderr = %q, want it to contain %q", stderr.String(), want)
	}
}

// TestRun_DispatchesRole verifies that the top-level dispatch reaches the role
// subcommand with the arguments that follow its name, and with the stream its
// caller passed. An unknown action is used because it is answered before the
// configuration is read, so the wiring is observable without a database: the
// quoted name in the output is what an invocation carrying the subcommand name
// along with it could not produce.
//
// [Ja] TestRun_DispatchesRole は、トップレベルの振り分けが、role サブコマンドへその名前に
// 続く引数と、呼び出し側が渡したストリームを伴って到達することを検証します。未知の動作を
// 使うのは、それが設定の読み込みより前に応答されるためで、データベース無しで配線を観測でき
// ます。出力に現れる引用符付きの名前は、サブコマンド名を一緒に引き渡してしまう実行では
// 作れないものです。
func TestRun_DispatchesRole(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"role", "give", "someone", "admin"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run() exit code = %d, want %d", code, exitUsage)
	}
	for _, want := range []string{`unknown role action: "give"`, "usage: groobb role grant|revoke <atname> <role>"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("run() stderr = %q, want it to contain %q", stderr.String(), want)
		}
	}
}
