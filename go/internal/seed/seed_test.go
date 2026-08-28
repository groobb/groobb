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

// runnerTestPassword is the password the roster a Runner test reads shares
// between its accounts. It differs from the one the roster tests use so that a
// digest checked here can only have come from the roster this test wrote.
//
// [Ja] runnerTestPassword は、Runner のテストが読み込む名簿がアカウント間で共有する
// パスワードです。名簿のテストが使うものと別にしているのは、ここで検証したダイジェストが、
// 本テストの書いた名簿からしか生まれ得ないようにするためです。
const runnerTestPassword = "shared-password-123"

// writeRunnerTestRoster writes the complete account roster a Runner test reads
// and returns its per-test path.
//
// [Ja] writeRunnerTestRoster は Runner のテストが読み込む完全なアカウント名簿を書き、
// テストごとのパスを返します。
func writeRunnerTestRoster(t *testing.T) string {
	t.Helper()

	return writeRoster(t, rosterWithPassword(runnerTestPassword))
}

// TestEnsureDevEnv verifies that development is the only environment a command
// handling development data is allowed in, and that an unset environment is
// refused rather than read as development.
//
// [Ja] TestEnsureDevEnv は、開発用データを扱うコマンドが許可される環境が開発環境だけで
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
		{name: "unset", env: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := EnsureDevEnv(tt.env)

			if tt.wantErr && err == nil {
				t.Errorf("EnsureDevEnv(%q) = nil, want an error", tt.env)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("EnsureDevEnv(%q) = %v, want nil", tt.env, err)
			}
		})
	}
}

// TestEnsureDevEnv_NamesTheInputItDoesNotRead verifies that the refusal says the
// configuration file is not consulted. Every other setting resolves from either
// input, so an operator who wrote the environment in the file would otherwise
// read the refusal as a claim that they had configured nothing.
//
// [Ja] TestEnsureDevEnv_NamesTheInputItDoesNotRead は、拒否の文言が、設定ファイルを
// 参照しないことを述べていることを検証します。他の設定はどちらの入力からも解決される
// ため、そう書かなければ、ファイルに環境を書いた運用者が拒否を「何も設定していない」
// という主張として読むことになります。
func TestEnsureDevEnv_NamesTheInputItDoesNotRead(t *testing.T) {
	t.Parallel()

	err := EnsureDevEnv("")
	if err == nil {
		t.Fatal("EnsureDevEnv(\"\") = nil, want an error")
	}
	for _, want := range []string{"APP_ENV", envFileKey} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("EnsureDevEnv(\"\") error = %q, want it to name %q", err, want)
		}
	}
}

// TestRunner_Run_RejectsANonDevelopmentEnvironment verifies that the guard
// answers before the run reaches the database.
//
// The Runner is given a nil database on purpose: the guard has to refuse before
// a single statement is issued, so a nil connection is enough here, and a guard
// that stopped working would panic instead of quietly emptying a database this
// test was never pointed at.
//
// [Ja] TestRunner_Run_RejectsANonDevelopmentEnvironment は、ガードが、実行が
// データベースへ到達する前に応答することを検証します。
//
// Runner には意図的に nil のデータベースを渡しています。ガードは文を 1 つも発行する前に
// 拒否しなければならないため、ここでは nil 接続で足ります。また、ガードが機能しなくなれば、
// 本テストが向いていたわけではないデータベースを黙って空にするのではなく panic します。
func TestRunner_Run_RejectsANonDevelopmentEnvironment(t *testing.T) {
	t.Parallel()

	err := NewRunner(nil, &config.Config{Env: "prod"}, io.Discard, matureProfile).Run(context.Background())

	if err == nil {
		t.Fatal("Run() should fail outside a development environment, but it succeeded")
	}
}

// TestRunner_Run verifies that a complete run replaces the managed rows with the
// accounts and the community content the screens are looked at with: the
// accounts can sign in, the one written to be read without an author has
// withdrawn, the boards hold conversations, the community is the one the profile
// names, no two-factor authentication setting is created, and every step reports its
// progress from zero through completion.
//
// [Ja] TestRunner_Run は、実行全体が管理対象の行を、画面を見るために使うアカウントと
// コミュニティの中身へ置き換えることを検証します。すなわち、アカウントがサインインでき、
// 作者抜きで読まれるために書かれたアカウントが退会しており、掲示板が会話を持ち、
// コミュニティがプロファイルの名指すものになり、二要素認証設定が作られず、どのステップも 0 件から完了までの
// 進捗を表示することです。
func TestRunner_Run(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	oldAtname := "beforeseed"
	testutil.NewUserBuilder(t, db).WithAtname(oldAtname).Build()

	// The community written here is what shows that a run replaces the row rather
	// than adding to it: the table holds at most the one row id 1, so a cleanup
	// that spared this one would leave the run unable to write its own.
	//
	// [Ja] ここで書くコミュニティは、実行がこの行を足すのではなく置き換えることを示します。
	// テーブルが持ちうるのは id 1 の 1 行だけであり、クリーンアップがこれを残せば、実行は
	// 自身の行を書けなくなります。
	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "Existing Community"); err != nil {
		t.Fatalf("failed to create the community: %v", err)
	}

	var out bytes.Buffer
	runner := NewRunner(db, &config.Config{Env: devEnv}, &out, matureProfile)
	runner.rosterPath = writeRunnerTestRoster(t)
	runner.profile = testProfile()

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	oldUser, err := userRepo.FindByAtname(ctx, oldAtname)
	if err != nil {
		t.Fatalf("FindByAtname(%q) error = %v", oldAtname, err)
	}
	if oldUser != nil {
		t.Errorf("the user present before Run() still exists with id %v", oldUser.ID)
	}

	var gotCommunityName string
	if err := db.Reader.QueryRowContext(ctx, "SELECT name FROM communities WHERE id = 1").Scan(&gotCommunityName); err != nil {
		t.Fatalf("failed to find the community after Run(): %v", err)
	}
	if gotCommunityName != matureCommunityName {
		t.Errorf("community name = %q, want %q", gotCommunityName, matureCommunityName)
	}

	// The accounts checked here are the ones a run leaves able to sign in. The
	// third account the roster names withdraws before the run ends, which is
	// checked below.
	//
	// [Ja] ここで確認するのは、実行がサインインできる状態で残すアカウントです。名簿が
	// 挙げる 3 つ目のアカウントは実行の終わりまでに退会するため、それは下で確認します。
	accounts := []struct {
		atname string
		email  string
	}{
		{atname: "seeduser1", email: "seeduser1@example.com"},
		{atname: "seeduser2", email: "seeduser2@example.com"},
	}
	passwordRepo := repository.NewUserPasswordRepository(db)
	for _, account := range accounts {
		user, err := userRepo.FindByAtname(ctx, account.atname)
		if err != nil {
			t.Fatalf("FindByAtname(%q) error = %v", account.atname, err)
		}
		if user == nil {
			t.Fatalf("no user was created for the atname %q", account.atname)
		}
		if user.Email != account.email {
			t.Errorf("user.Email = %q, want %q", user.Email, account.email)
		}

		password, err := passwordRepo.FindByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("FindByUserID(%v) error = %v", user.ID, err)
		}
		if password == nil {
			t.Fatalf("no password credential was created for the atname %q", account.atname)
		}
		if err := auth.CheckPassword(password.PasswordDigest, runnerTestPassword); err != nil {
			t.Errorf("the password of %q does not match the roster password: %v", account.atname, err)
		}
	}

	// The withdrawn account is still a row, so it is counted with the others and
	// read back through a query of its own: every lookup the application makes
	// leaves a withdrawn account out.
	//
	// [Ja] 退会したアカウントも行としては残るため、他と合わせて数え、専用のクエリで
	// 読み直します。アプリケーションが行うルックアップは、いずれも退会済みアカウントを
	// 除外するためです。
	var userCount int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if userCount != len(accounts)+1 {
		t.Errorf("user count = %d, want %d", userCount, len(accounts)+1)
	}

	var withdrawnID model.UserID
	var withdrawnEmail, withdrawnAtname string
	if err := db.Reader.QueryRowContext(ctx,
		"SELECT id, email, atname FROM users WHERE deleted_at IS NOT NULL",
	).Scan(&withdrawnID, &withdrawnEmail, &withdrawnAtname); err != nil {
		t.Fatalf("failed to find the withdrawn account after Run(): %v", err)
	}
	if want := model.AnonymizedEmail(withdrawnID); withdrawnEmail != want {
		t.Errorf("the withdrawn account's email = %q, want %q", withdrawnEmail, want)
	}
	if want := model.AnonymizedAtname(withdrawnID); withdrawnAtname != want {
		t.Errorf("the withdrawn account's atname = %q, want %q", withdrawnAtname, want)
	}

	var twoFactorAuthCount int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_two_factor_auths").Scan(&twoFactorAuthCount); err != nil {
		t.Fatalf("failed to count two-factor authentication settings: %v", err)
	}
	if twoFactorAuthCount != 0 {
		t.Errorf("two-factor authentication setting count = %d, want 0", twoFactorAuthCount)
	}

	for table, want := range map[string]int{
		"categories": len(matureCategories),
		"boards":     len(matureBoards),
	} {
		var count int
		if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("failed to count the rows of %s: %v", table, err)
		}
		if count != want {
			t.Errorf("%s count = %d, want %d", table, count, want)
		}
	}

	for _, table := range []string{"threads", "posts", "post_references"} {
		var count int
		if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("failed to count the rows of %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("%s count = 0, want the run to have written some", table)
		}
	}

	wantProgress := []string{
		"\r  community 1/1\n",
		"\r  users 0/3",
		"\r  users 3/3\n",
		fmt.Sprintf("\r  boards 0/%d", len(matureBoards)),
		"\r  threads 0/",
		"\r  withdrawal 1/1\n",
	}
	for _, want := range wantProgress {
		if !strings.Contains(out.String(), want) {
			t.Errorf("Run() progress = %q, want it to contain %q", out.String(), want)
		}
	}
}

// TestRunner_Run_RollsBackCleanupWhenGenerationFails verifies that cleanup and
// generation share one transaction: an insert failure restores the managed rows
// that existed before the run.
//
// [Ja] TestRunner_Run_RollsBackCleanupWhenGenerationFails は、クリーンアップと生成が
// 1 つのトランザクションを共有し、INSERT の失敗時に実行前の管理対象行が復元されることを
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
		t.Fatalf("failed to create the preexisting password: %v", err)
	}

	if _, err := db.Writer.ExecContext(ctx, `
		CREATE TRIGGER fail_seed_user_insert
		BEFORE INSERT ON users
		BEGIN
			SELECT RAISE(FAIL, 'forced user insert failure');
		END
	`); err != nil {
		t.Fatalf("failed to create the failure trigger: %v", err)
	}

	runner := NewRunner(db, &config.Config{Env: devEnv}, io.Discard, matureProfile)
	runner.rosterPath = writeRunnerTestRoster(t)

	err := runner.Run(ctx)
	if err == nil {
		t.Fatal("Run() should fail when inserting a generated user, but it succeeded")
	}
	if !strings.Contains(err.Error(), "forced user insert failure") {
		t.Errorf("Run() error = %q, want the forced insert failure", err)
	}

	user, err := repository.NewUserRepository(db).FindByAtname(ctx, oldAtname)
	if err != nil {
		t.Fatalf("FindByAtname(%q) error = %v", oldAtname, err)
	}
	if user == nil {
		t.Fatal("the user present before Run() was not restored by rollback")
	}
	if user.ID != oldUserID {
		t.Errorf("restored user id = %v, want %v", user.ID, oldUserID)
	}

	password, err := passwordRepo.FindByUserID(ctx, oldUserID)
	if err != nil {
		t.Fatalf("FindByUserID(%v) error = %v", oldUserID, err)
	}
	if password == nil {
		t.Fatal("the password present before Run() was not restored by rollback")
	}
	if password.PasswordDigest != oldDigest {
		t.Errorf("restored password digest = %q, want %q", password.PasswordDigest, oldDigest)
	}
}
