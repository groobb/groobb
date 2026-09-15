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

// setUpRoleDatabaseはテストにマイグレーション済みのデータベースを与え、サブコマンドが
// 読み込む設定をそこへ向け、テストがそれを読み戻すためのプールを返します。
//
// サブコマンドはファイルを自分で開くため、テストが持つのは実行が使うプールではなく、同じ
// データベースに対する2組目のプールです。これによって、サブコマンドが書き込んだ行を、
// プロセス自身の見え方の外から検証できます。
func setUpRoleDatabase(t *testing.T) *database.DB {
	t.Helper()

	path := testutil.SetupDBPath(t)
	setDatabaseEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", path)

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("検証用のデータベースのオープンに失敗: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("検証用のデータベースのクローズに失敗: %v", err)
		}
	})

	return db
}

// newRoleLoggerは、テストがサブコマンドへ渡すloggerを返します。実行が報告する
// 診断が、検証の手が届かないテストプロセス自身の標準エラーではなく、wに届くようにする
// ためです。
func newRoleLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil))
}

// assertHoldsAdminは、ユーザーが組み込みのadminロールを持つかどうかを検証します。
//
// 読み取りは、アプリケーションが操作者の権限を解決するのに使うリポジトリを通します。検査の
// 対象を、アプリケーションから見える割当そのものにするためです。
func assertHoldsAdmin(t *testing.T, db *database.DB, userID model.UserID, wantHolds bool) {
	t.Helper()

	roles, err := repository.NewRoleRepository(db).ListByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUserID(%v)のエラー = %v", userID, err)
	}

	holds := false
	for _, role := range roles {
		if role.Name == model.RoleNameAdmin {
			holds = true
		}
	}

	if holds != wantHolds {
		t.Errorf("ユーザーが %s ロールを持つか = %v、期待値 = %v", model.RoleNameAdmin, holds, wantHolds)
	}
}

// TestRunRole_RejectsInvalidArgumentsは、本サブコマンドが動作・atname・ロールを
// 要求し、それ以外をusageと使用方法の誤りを示す終了コードで応答すること、そして未知の
// 動作が引用符付きで出力され、打ち間違いが見えることを検証します。
func TestRunRole_RejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name:         "引数が無い",
			wantContains: []string{"usage: groobb role grant|revoke <atname> <role>"},
		},
		{
			name:         "ロールの指定が無い",
			args:         []string{"grant", "someone"},
			wantContains: []string{"usage: groobb role grant|revoke <atname> <role>"},
		},
		{
			name:         "引数が多すぎる",
			args:         []string{"grant", "someone", "admin", "admin"},
			wantContains: []string{"usage: groobb role grant|revoke <atname> <role>"},
		},
		{
			name:         "未知の動作",
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
				t.Errorf("runRole()の終了コード = %d、期待値 = %d", code, exitUsage)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("runRole()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
				}
			}
		})
	}
}

// TestRunRole_GrantsAndRevokesTheRoleは、実データベースに対して2つの方向を検証
// します。atnameで名指ししたアカウントがロールを持つ状態になり、2度目の実行がそれを再び
// 取り上げることです。
//
// 先に別の管理者を作るのは、コミュニティが常に管理者を1人以上持つためです。そうしなければ
// 剥奪は実行されず、その理由で拒否されることになります。
func TestRunRole_GrantsAndRevokesTheRole(t *testing.T) {
	db := setUpRoleDatabase(t)

	atname := "roleholder"
	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(testutil.NewUserBuilder(t, db).Build()).Build()

	var stderr bytes.Buffer

	if code := runRole(context.Background(), []string{"grant", atname, string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr)); code != 0 {
		t.Fatalf("runRole(grant)の終了コード = %d、期待値 = 0 (標準エラー出力: %q)", code, stderr.String())
	}
	assertHoldsAdmin(t, db, userID, true)

	if code := runRole(context.Background(), []string{"revoke", atname, string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr)); code != 0 {
		t.Fatalf("runRole(revoke)の終了コード = %d、期待値 = 0 (標準エラー出力: %q)", code, stderr.String())
	}
	assertHoldsAdmin(t, db, userID, false)
}

// TestRunRole_RefusesToRevokeTheLastAdministratorは、UseCaseが課す保護がコマンド
// ラインまで届くことを検証します。最後の保持者はロールを持ったままになり、その失敗は終了
// コード1によって使用方法の誤りと区別されます。
func TestRunRole_RefusesToRevokeTheLastAdministrator(t *testing.T) {
	db := setUpRoleDatabase(t)

	atname := "lastadmin"
	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

	var stderr bytes.Buffer

	code := runRole(context.Background(), []string{"revoke", atname, string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr))

	if code != 1 {
		t.Errorf("runRole(revoke)の終了コード = %d、期待値 = 1", code)
	}
	if want := "最後の管理者からの剥奪"; !strings.Contains(stderr.String(), want) {
		t.Errorf("runRole(revoke)の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
	}
	assertHoldsAdmin(t, db, userID, true)
}

// TestRunRole_ReportsANameThatCarriesNothingは、どのアカウントも持たないatnameと、
// どのロールも持たないロール名が、どちらもコマンドラインの誤りではなく依頼された処理の失敗に
// なることを検証します。終了コードは1であり、実行が名指ししたアカウントは何も得ません。
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
			name:        "どのアカウントも持たないatname",
			args:        []string{"grant", "absentuser", string(model.RoleNameAdmin)},
			wantMessage: `no user has the atname \"absentuser\"`,
		},
		{
			name:        "どのロールも持たないロール名",
			args:        []string{"grant", atname, "moderator"},
			wantMessage: "ロールが見つからない: role_name=moderator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer

			code := runRole(context.Background(), tt.args, &stderr, newRoleLogger(&stderr))

			if code != 1 {
				t.Errorf("runRole()の終了コード = %d、期待値 = 1", code)
			}
			if !strings.Contains(stderr.String(), tt.wantMessage) {
				t.Errorf("runRole()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), tt.wantMessage)
			}
			assertHoldsAdmin(t, db, userID, false)
		})
	}
}

// TestRunRole_ReturnsFailureExitCodeは、データベースへ到達できない失敗が終了コード1
// によって使用方法の誤りと区別されること、そして診断がサブコマンドの行っていたことを
// 名指しすることを検証します。
//
// これはmodel.AppErrorとしては届かない失敗であり、報告のもう一方の分岐へ到達するのは
// これです。上のテストがどれも通らない分岐です。
func TestRunRole_ReturnsFailureExitCode(t *testing.T) {
	setDatabaseEnv(t)
	t.Setenv("GROOBB_DATABASE_PATH", filepath.Join(t.TempDir(), "missing", "groobb.sqlite"))

	var stderr bytes.Buffer

	code := runRole(context.Background(), []string{"grant", "someone", string(model.RoleNameAdmin)}, &stderr, newRoleLogger(&stderr))

	if code != 1 {
		t.Errorf("runRole()の終了コード = %d、期待値 = 1", code)
	}
	if want := "failed to change the role of the user"; !strings.Contains(stderr.String(), want) {
		t.Errorf("runRole()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
	}
}

// TestRun_DispatchesRoleは、トップレベルの振り分けが、roleサブコマンドへその名前に
// 続く引数と、呼び出し側が渡したストリームを伴って到達することを検証します。未知の動作を
// 使うのは、それが設定の読み込みより前に応答されるためで、データベース無しで配線を観測でき
// ます。出力に現れる引用符付きの名前は、サブコマンド名を一緒に引き渡してしまう実行では
// 作れないものです。
func TestRun_DispatchesRole(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	code := run([]string{"role", "give", "someone", "admin"}, io.Discard, &stderr)

	if code != exitUsage {
		t.Errorf("run()の終了コード = %d、期待値 = %d", code, exitUsage)
	}
	for _, want := range []string{`unknown role action: "give"`, "usage: groobb role grant|revoke <atname> <role>"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("run()の標準エラー出力 = %q、%q を含むことを期待", stderr.String(), want)
		}
	}
}
