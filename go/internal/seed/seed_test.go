package seed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// runnerTestPasswordは、Runnerのテストが読み込む名簿がアカウント間で共有する
// パスワードです。名簿のテストが使うものと別にしているのは、ここで検証したダイジェストが、
// 本テストの書いた名簿からしか生まれ得ないようにするためです。
const runnerTestPassword = "shared-password-123"

// writeRunnerTestRosterはRunnerのテストが読み込む完全なアカウント名簿を書き、
// テストごとのパスを返します。
func writeRunnerTestRoster(t *testing.T) string {
	t.Helper()

	return writeRoster(t, rosterWithPassword(runnerTestPassword))
}

// TestEnsureDevEnvは、開発用データを扱うコマンドが許可される環境が開発環境だけで
// あること、そして未設定の環境が開発環境として読まれるのではなく拒否されることを検証
// します。
func TestEnsureDevEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     string
		wantErr bool
	}{
		{name: "development", env: "dev", wantErr: false},
		{name: "test", env: "test", wantErr: true},
		{name: "production", env: "prod", wantErr: true},
		{name: "未設定", env: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := EnsureDevEnv(tt.env)

			if tt.wantErr && err == nil {
				t.Errorf("EnsureDevEnv(%q) = nil、エラーを期待", tt.env)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("EnsureDevEnv(%q) = %v、期待値 = nil", tt.env, err)
			}
		})
	}
}

// TestEnsureDevEnv_NamesTheInputItDoesNotReadは、拒否の文言が、設定ファイルを
// 参照しないことを述べていることを検証します。他の設定はどちらの入力からも解決される
// ため、そう書かなければ、ファイルに環境を書いた運用者が拒否を「何も設定していない」
// という主張として読むことになります。
func TestEnsureDevEnv_NamesTheInputItDoesNotRead(t *testing.T) {
	t.Parallel()

	err := EnsureDevEnv("")
	if err == nil {
		t.Fatal("EnsureDevEnv(\"\") = nil、エラーを期待")
	}
	for _, want := range []string{"APP_ENV", envFileKey} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("EnsureDevEnv(\"\")のエラー = %q、%q を名指すことを期待", err, want)
		}
	}
}

// TestRunner_Run_RejectsANonDevelopmentEnvironmentは、ガードが、実行が
// データベースへ到達する前に応答することを検証します。
//
// Runnerには意図的にnilのデータベースを渡しています。ガードは文を1つも発行する前に
// 拒否しなければならないため、ここではnil接続で足ります。また、ガードが機能しなくなれば、
// 本テストが向いていたわけではないデータベースを黙って空にするのではなくpanicします。
func TestRunner_Run_RejectsANonDevelopmentEnvironment(t *testing.T) {
	t.Parallel()

	err := NewRunner(nil, &config.Config{Env: "prod"}, io.Discard, matureProfile).Run(context.Background())

	if err == nil {
		t.Fatal("開発環境の外でRun()が失敗することを期待したが、成功した")
	}
}

// TestRunner_Runは、実行全体が管理対象の行を、画面を見るために使うアカウントと
// コミュニティの中身へ置き換えることを検証します。すなわち、アカウントがサインインでき、
// 作者抜きで読まれるために書かれたアカウントが退会しており、掲示板が会話を持ち、
// コミュニティがプロファイルの名指すものになり、二要素認証設定が作られず、どのステップも0件から完了までの
// 進捗を表示することです。
func TestRunner_Run(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	oldAtname := "beforeseed"
	testutil.NewUserBuilder(t, db).WithAtname(oldAtname).Build()

	// ここで書くコミュニティは、実行がこの行を足すのではなく置き換えることを示します。
	// テーブルが持ちうるのはid 1の1行だけであり、クリーンアップがこれを残せば、実行は
	// 自身の行を書けなくなります。
	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "Existing Community"); err != nil {
		t.Fatalf("コミュニティの作成に失敗: %v", err)
	}

	var out bytes.Buffer
	runner := NewRunner(db, &config.Config{Env: devEnv}, &out, matureProfile)
	runner.rosterPath = writeRunnerTestRoster(t)
	runner.profile = testProfile()

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run()のエラー = %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	oldUser, err := userRepo.FindByAtname(ctx, oldAtname)
	if err != nil {
		t.Fatalf("FindByAtname(%q)のエラー = %v", oldAtname, err)
	}
	if oldUser != nil {
		t.Errorf("Run()の前からいたユーザーがID %v のまま残っている", oldUser.ID)
	}

	var gotCommunityName string
	if err := db.Reader.QueryRowContext(ctx, "SELECT name FROM communities WHERE id = 1").Scan(&gotCommunityName); err != nil {
		t.Fatalf("Run()の後のコミュニティの取得に失敗: %v", err)
	}
	if gotCommunityName != matureCommunityName {
		t.Errorf("コミュニティ名 = %q、期待値 = %q", gotCommunityName, matureCommunityName)
	}

	// ここで確認するのは、実行がサインインできる状態で残すアカウントです。名簿が
	// 挙げる3つ目のアカウントは実行の終わりまでに退会するため、それは下で確認します。
	accounts := []struct {
		atname string
		email  string
	}{
		{atname: "seeduser1", email: "seeduser1@example.com"},
		{atname: "seeduser2", email: "seeduser2@example.com"},
		{atname: "seeduser4", email: "seeduser4@example.com"},
	}
	passwordRepo := repository.NewUserPasswordRepository(db)
	for _, account := range accounts {
		user, err := userRepo.FindByAtname(ctx, account.atname)
		if err != nil {
			t.Fatalf("FindByAtname(%q)のエラー = %v", account.atname, err)
		}
		if user == nil {
			t.Fatalf("atname %q のユーザーが作成されていない", account.atname)
		}
		if user.Email != account.email {
			t.Errorf("user.Email = %q、期待値 = %q", user.Email, account.email)
		}

		password, err := passwordRepo.FindByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("FindByUserID(%v)のエラー = %v", user.ID, err)
		}
		if password == nil {
			t.Fatalf("atname %q のパスワードが作成されていない", account.atname)
		}
		if err := auth.CheckPassword(password.PasswordDigest, runnerTestPassword); err != nil {
			t.Errorf("%q のパスワードが名簿のパスワードと一致しない: %v", account.atname, err)
		}
	}

	// 名簿がadminの役割を与えるアカウントは、実行が終わった時点でそのロールを持ちます。
	// サブコマンドで先に誰かを任命しなくても管理画面を開けるのはこれによります。読み戻しは、
	// アプリケーションが操作者の権限を解決するのに使うリポジトリを通します。検査の対象を、
	// アプリケーションから見える割当そのものにするためです。
	adminUser, err := userRepo.FindByAtname(ctx, "seeduser4")
	if err != nil {
		t.Fatalf("FindByAtname(%q)のエラー = %v", "seeduser4", err)
	}
	if adminUser == nil {
		t.Fatal("管理用アカウントのユーザーが作成されていない")
	}

	adminRoles, err := repository.NewRoleRepository(db).ListByUserID(ctx, adminUser.ID)
	if err != nil {
		t.Fatalf("ListByUserID(%v)のエラー = %v", adminUser.ID, err)
	}
	if len(adminRoles) != 1 || adminRoles[0].Name != model.RoleNameAdmin {
		t.Errorf("管理用アカウントのロール = %v、期待値は %s だけ", adminRoles, model.RoleNameAdmin)
	}

	// 他の誰もロールを持ちません。他の画面を眺めるのに使うアカウントは一般の利用者で
	// あり、ロールをより広く配る実行は、どの画面にも管理画面へのリンクを載せたまま残すことに
	// なります。
	var userRoleCount int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_roles").Scan(&userRoleCount); err != nil {
		t.Fatalf("ロールの割り当ての件数の取得に失敗: %v", err)
	}
	if userRoleCount != 1 {
		t.Errorf("ロールの割り当ての件数 = %d、期待値 = 1", userRoleCount)
	}

	// 退会したアカウントも行としては残るため、他と合わせて数え、専用のクエリで
	// 読み直します。アプリケーションが行うルックアップは、いずれも退会済みアカウントを
	// 除外するためです。
	var userCount int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		t.Fatalf("ユーザーの件数の取得に失敗: %v", err)
	}
	if userCount != len(accounts)+1 {
		t.Errorf("ユーザーの件数 = %d、期待値 = %d", userCount, len(accounts)+1)
	}

	var withdrawnID model.UserID
	var withdrawnEmail, withdrawnAtname string
	if err := db.Reader.QueryRowContext(ctx,
		"SELECT id, email, atname FROM users WHERE deleted_at IS NOT NULL",
	).Scan(&withdrawnID, &withdrawnEmail, &withdrawnAtname); err != nil {
		t.Fatalf("Run()の後の退会済みアカウントの取得に失敗: %v", err)
	}
	if want := model.AnonymizedEmail(withdrawnID); withdrawnEmail != want {
		t.Errorf("退会済みアカウントのemail = %q、期待値 = %q", withdrawnEmail, want)
	}
	if want := model.AnonymizedAtname(withdrawnID); withdrawnAtname != want {
		t.Errorf("退会済みアカウントのatname = %q、期待値 = %q", withdrawnAtname, want)
	}

	var twoFactorAuthCount int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_two_factor_auths").Scan(&twoFactorAuthCount); err != nil {
		t.Fatalf("2段階認証の設定の件数の取得に失敗: %v", err)
	}
	if twoFactorAuthCount != 0 {
		t.Errorf("2段階認証の設定の件数 = %d、期待値 = 0", twoFactorAuthCount)
	}

	for table, want := range map[string]int{
		"categories": len(matureCategories),
		"boards":     len(matureBoards),
	} {
		var count int
		if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("%s の行数の取得に失敗: %v", table, err)
		}
		if count != want {
			t.Errorf("%s の件数 = %d、期待値 = %d", table, count, want)
		}
	}

	for _, table := range []string{"threads", "posts", "post_references"} {
		var count int
		if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("%s の行数の取得に失敗: %v", table, err)
		}
		if count == 0 {
			t.Errorf("%s の件数 = 0、実行が何件か書き込んでいることを期待", table)
		}
	}

	wantProgress := []string{
		"\r  community 1/1\n",
		"\r  users 0/4",
		"\r  users 4/4\n",
		"\r  user roles 1/1\n",
		fmt.Sprintf("\r  boards 0/%d", len(matureBoards)),
		"\r  threads 0/",
		"\r  withdrawal 1/1\n",
	}
	for _, want := range wantProgress {
		if !strings.Contains(out.String(), want) {
			t.Errorf("Run()の進捗表示 = %q、%q を含むことを期待", out.String(), want)
		}
	}
}

// TestRunner_Run_RollsBackCleanupWhenGenerationFailsは、クリーンアップと生成が
// 1つのトランザクションを共有し、INSERTの失敗時に実行前の管理対象行が復元されることを
// 検証します。
func TestRunner_Run_RollsBackCleanupWhenGenerationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	oldAtname := "beforefailedseed"
	oldUserID := testutil.NewUserBuilder(t, db).WithAtname(oldAtname).Build()

	const oldDigest = "digest-before-the-failed-seed"
	passwordRepo := repository.NewUserPasswordRepository(db)
	if _, err := passwordRepo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         oldUserID,
		PasswordDigest: oldDigest,
	}); err != nil {
		t.Fatalf("既存のパスワードの作成に失敗: %v", err)
	}

	if _, err := db.Writer.ExecContext(ctx, `
		CREATE TRIGGER fail_seed_user_insert
		BEFORE INSERT ON users
		BEGIN
			SELECT RAISE(FAIL, 'forced user insert failure');
		END
	`); err != nil {
		t.Fatalf("失敗させるトリガーの作成に失敗: %v", err)
	}

	runner := NewRunner(db, &config.Config{Env: devEnv}, io.Discard, matureProfile)
	runner.rosterPath = writeRunnerTestRoster(t)

	err := runner.Run(ctx)
	if err == nil {
		t.Fatal("生成したユーザーの挿入でRun()が失敗することを期待したが、成功した")
	}
	if !strings.Contains(err.Error(), "forced user insert failure") {
		t.Errorf("Run()のエラー = %q、強制した挿入の失敗を期待", err)
	}

	user, err := repository.NewUserRepository(db).FindByAtname(ctx, oldAtname)
	if err != nil {
		t.Fatalf("FindByAtname(%q)のエラー = %v", oldAtname, err)
	}
	if user == nil {
		t.Fatal("Run()の前からいたユーザーがロールバックで復元されていない")
	}
	if user.ID != oldUserID {
		t.Errorf("復元されたユーザーのID = %v、期待値 = %v", user.ID, oldUserID)
	}

	password, err := passwordRepo.FindByUserID(ctx, oldUserID)
	if err != nil {
		t.Fatalf("FindByUserID(%v)のエラー = %v", oldUserID, err)
	}
	if password == nil {
		t.Fatal("Run()の前からあったパスワードがロールバックで復元されていない")
	}
	if password.PasswordDigest != oldDigest {
		t.Errorf("復元されたパスワードダイジェスト = %q、期待値 = %q", password.PasswordDigest, oldDigest)
	}
}
