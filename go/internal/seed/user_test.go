package seed

import (
	"context"
	"database/sql"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// testPasswordDigestは、名簿が共通パスワードをハッシュ化して得るダイジェストの
// 代わりです。ここでは固定の文字列で足り、実行がどの行を書き込むかを対象とし、パスワードの
// ハッシュ化方法は対象としない本テストからbcryptを外せます。検証するのは、名簿が運ぶ
// ダイジェストがアカウントの保存するダイジェストであることです。
const testPasswordDigest = "digest-of-the-shared-password"

// testRosterはgenerateUsersのテストが使うアカウントを返します。
func testRoster() *userRoster {
	return &userRoster{
		path:           rosterPath,
		passwordDigest: testPasswordDigest,
		users: []rosterUser{
			{role: roleStarter, atname: "seeduser1", email: "seeduser1@example.com", note: "opens threads"},
			{role: roleReplier, atname: "seeduser2", email: "seeduser2@example.com", note: "replies to them"},
			{role: roleAdmin, atname: "seeduser4", email: "seeduser4@example.com", note: "administers the community"},
			{role: roleWithdrawn, atname: "seeduser3", email: "seeduser3@example.com", note: "withdraws"},
		},
	}
}

// newTestRunnerは進捗の出力を捨てるRunnerを返します。実行全体ではなく生成器を
// 1つ呼ぶテストのためのものです。
func newTestRunner(db *database.DB) *Runner {
	return NewRunner(db, &config.Config{Env: devEnv}, io.Discard, matureProfile)
}

// beginTxはテスト用に書き込みトランザクションを開始し、テストの終了時に
// ロールバックします。コミットの前に失敗したテストが、開いたままのものを残さないように
// するためです。
func beginTx(t *testing.T, db *database.DB) *sql.Tx {
	t.Helper()

	tx, err := db.Writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("トランザクションの開始に失敗: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	return tx
}

// TestRunner_GenerateUsersは、名簿が挙げるすべてのアカウントが、サインインに使う
// パスワード資格情報を伴うユーザーとして書き込まれること、そして生成器がそれらのアカウント
// を後続の生成器へ役割で引き渡すことを検証します。
func TestRunner_GenerateUsers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	roster := testRoster()
	st := &state{roster: roster}

	tx := beginTx(t, db)
	if err := newTestRunner(db).generateUsers(ctx, tx, st); err != nil {
		t.Fatalf("generateUsers()のエラー = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("トランザクションのコミットに失敗: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)

	for _, account := range roster.users {
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
		if user.Locale != model.DefaultLocale {
			t.Errorf("user.Locale = %q、期待値 = %q", user.Locale, model.DefaultLocale)
		}
		if user.TimeZone != seedUserTimeZone {
			t.Errorf("user.TimeZone = %q、期待値 = %q", user.TimeZone, seedUserTimeZone)
		}

		// どのアカウントもサインインできるアカウントとして作成します。生成器が
		// アカウントを求めるときの役割が述べるのは、そのアカウントが何を見せるためにいるか
		// であり、どの状態で作成されるかではありません。
		if user.DeletedAt != nil {
			t.Errorf("user.DeletedAt = %v、期待値 = nil", user.DeletedAt)
		}

		password, err := userPasswordRepo.FindByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("FindByUserID()のエラー = %v", err)
		}
		if password == nil {
			t.Fatalf("atname %q のパスワードが作成されていない", account.atname)
		}
		if password.PasswordDigest != testPasswordDigest {
			t.Errorf("password.PasswordDigest = %q、期待値 = %q", password.PasswordDigest, testPasswordDigest)
		}

		seeded := st.users.user(account.role)
		if seeded == nil {
			t.Fatalf("ロール %s のアカウントが引き渡されていない", account.role)
		}
		if seeded.ID != user.ID {
			t.Errorf("ロール %s として引き渡されたアカウントのID = %v、期待値 = %v", account.role, seeded.ID, user.ID)
		}
	}
}

// TestRunner_GenerateUsers_ReportsAnAccountItCannotCreateは、atnameの衝突が、
// 成功したように見える実行からそのアカウントだけを欠落させるのではなく、そのアカウントを
// 名指しして生成器を止めることを検証します。
func TestRunner_GenerateUsers_ReportsAnAccountItCannotCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	roster := testRoster()

	testutil.NewUserBuilder(t, db).WithAtname(roster.users[0].atname).Build()

	err := newTestRunner(db).generateUsers(ctx, beginTx(t, db), &state{roster: roster})
	if err == nil {
		t.Fatal("atnameが使用済みのときにgenerateUsers()が失敗することを期待したが、成功した")
	}
	if !strings.Contains(err.Error(), roster.users[0].atname) {
		t.Errorf("generateUsers()のエラー = %q、作成できなかったアカウントを名指すことを期待", err)
	}
}

// TestRunner_GenerateWithdrawalは、投稿を作者抜きで読むためのアカウントが、
// アプリケーションが退会させるのと同じやり方で退会させられること (論理削除され、持って
// いたemailとatnameが解放されること)、そして会話を読むためのアカウントがサインイン
// できる状態で残されることを検証します。
func TestRunner_GenerateWithdrawal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	st := &state{roster: testRoster()}

	tx := beginTx(t, db)
	runner := newTestRunner(db)
	if err := runner.generateUsers(ctx, tx, st); err != nil {
		t.Fatalf("generateUsers()のエラー = %v", err)
	}
	if err := runner.generateWithdrawal(ctx, tx, st); err != nil {
		t.Fatalf("generateWithdrawal()のエラー = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("トランザクションのコミットに失敗: %v", err)
	}

	withdrawn := st.users.user(roleWithdrawn)
	if withdrawn == nil {
		t.Fatal("退会するロールのアカウントが作成されていない")
	}

	// 行はルックアップではなく直接読みます。ルックアップは、退会済みアカウントを
	// 設計上除外するためです。
	var deletedAt *time.Time
	var email, atname string
	if err := db.Reader.QueryRowContext(ctx,
		"SELECT deleted_at, email, atname FROM users WHERE id = ?", int64(withdrawn.ID),
	).Scan(&deletedAt, &email, &atname); err != nil {
		t.Fatalf("退会済みアカウントの読み取りに失敗: %v", err)
	}

	if deletedAt == nil {
		t.Error("退会済みアカウントのdeleted_atが無い (退会した時刻を期待)")
	}
	if want := model.AnonymizedEmail(withdrawn.ID); email != want {
		t.Errorf("退会済みアカウントのemail = %q、期待値 = %q", email, want)
	}
	if want := model.AnonymizedAtname(withdrawn.ID); atname != want {
		t.Errorf("退会済みアカウントのatname = %q、期待値 = %q", atname, want)
	}

	userRepo := repository.NewUserRepository(db)
	for _, role := range []seedRole{roleStarter, roleReplier} {
		account := st.users.user(role)
		if account == nil {
			t.Fatalf("ロール %s のアカウントが作成されていない", role)
		}

		user, err := userRepo.FindByID(ctx, account.ID)
		if err != nil {
			t.Fatalf("FindByID(%v)のエラー = %v", account.ID, err)
		}
		if user == nil {
			t.Errorf("ロール %s のアカウントが引けなくなっている (サインインできるまま残ることを期待)", role)
		}
	}
}

// TestSignInRolesは、コマンドがusageを組み立てる元にする一覧が、完了した実行が
// サインイン可能なまま残すアカウントの役割だけを持つことを検証します。withdrawnも生成器の
// 役割ですが、usageに挙げると、完了したシードが無効化したアカウントの指定を促すことに
// なります。
func TestSignInRoles(t *testing.T) {
	t.Parallel()

	roles := SignInRoles()

	if want := []string{string(roleStarter), string(roleReplier), string(roleAdmin)}; !slices.Equal(roles, want) {
		t.Errorf("SignInRoles() = %v、期待値 = %v", roles, want)
	}

	// 挙げる役割はすべて、名簿が持つことを要求される役割である必要があります。
	// そうでなければ、usageが、どのアカウントも応じない役割を挙げることになります。
	for _, role := range roles {
		if !slices.Contains(allSeedRoles, seedRole(role)) {
			t.Errorf("SignInRoles()が %q を示している (生成器が知るロールを期待)", role)
		}
	}
}
