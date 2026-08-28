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
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// testPasswordDigest stands in for the digest the roster hashes its shared
// password into. A fixed string is enough here and keeps bcrypt out of a test
// that is about which rows a run writes rather than about how a password is
// hashed: what it verifies is that the digest the roster carries is the digest
// the account stores.
//
// [Ja] testPasswordDigest は、名簿が共通パスワードをハッシュ化して得るダイジェストの
// 代わりです。ここでは固定の文字列で足り、実行がどの行を書き込むかを対象とし、パスワードの
// ハッシュ化方法は対象としない本テストから bcrypt を外せます。検証するのは、名簿が運ぶ
// ダイジェストがアカウントの保存するダイジェストであることです。
const testPasswordDigest = "digest-of-the-shared-password"

// testRoster returns the accounts a generateUsers test works from.
//
// [Ja] testRoster は generateUsers のテストが使うアカウントを返します。
func testRoster() *userRoster {
	return &userRoster{
		path:           rosterPath,
		passwordDigest: testPasswordDigest,
		users: []rosterUser{
			{role: roleStarter, atname: "seeduser1", email: "seeduser1@example.com", note: "opens threads"},
			{role: roleReplier, atname: "seeduser2", email: "seeduser2@example.com", note: "replies to them"},
			{role: roleWithdrawn, atname: "seeduser3", email: "seeduser3@example.com", note: "withdraws"},
		},
	}
}

// newTestRunner returns a Runner that discards its progress output, for a test
// that calls one generator rather than a whole run.
//
// [Ja] newTestRunner は進捗の出力を捨てる Runner を返します。実行全体ではなく生成器を
// 1 つ呼ぶテストのためのものです。
func newTestRunner(db *database.DB) *Runner {
	return NewRunner(db, &config.Config{Env: devEnv}, io.Discard, matureProfile)
}

// beginTx starts a write transaction for the test and rolls it back when the
// test ends, so that a test that fails before committing leaves nothing open.
//
// [Ja] beginTx はテスト用に書き込みトランザクションを開始し、テストの終了時に
// ロールバックします。コミットの前に失敗したテストが、開いたままのものを残さないように
// するためです。
func beginTx(t *testing.T, db *database.DB) *sql.Tx {
	t.Helper()

	tx, err := db.Writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to begin the transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	return tx
}

// TestRunner_GenerateUsers verifies that every account the roster names is
// written as a user with the password credential that signs it in, and that the
// generator hands the accounts on by role for the generators that follow it.
//
// [Ja] TestRunner_GenerateUsers は、名簿が挙げるすべてのアカウントが、サインインに使う
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
		t.Fatalf("generateUsers() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit the transaction: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)

	for _, account := range roster.users {
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
		if user.Locale != i18n.DefaultLang {
			t.Errorf("user.Locale = %q, want %q", user.Locale, i18n.DefaultLang)
		}
		if user.TimeZone != seedUserTimeZone {
			t.Errorf("user.TimeZone = %q, want %q", user.TimeZone, seedUserTimeZone)
		}

		// Every account is created as one that can sign in. The role a generator
		// asks for an account by says what it is there to show, not what state
		// it is created in.
		//
		// [Ja] どのアカウントもサインインできるアカウントとして作成します。生成器が
		// アカウントを求めるときの役割が述べるのは、そのアカウントが何を見せるためにいるか
		// であり、どの状態で作成されるかではありません。
		if user.DeletedAt != nil {
			t.Errorf("user.DeletedAt = %v, want nil", user.DeletedAt)
		}

		password, err := userPasswordRepo.FindByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("FindByUserID() error = %v", err)
		}
		if password == nil {
			t.Fatalf("no password credential was created for the atname %q", account.atname)
		}
		if password.PasswordDigest != testPasswordDigest {
			t.Errorf("password.PasswordDigest = %q, want %q", password.PasswordDigest, testPasswordDigest)
		}

		seeded := st.users.user(account.role)
		if seeded == nil {
			t.Fatalf("no account was handed on for the role %s", account.role)
		}
		if seeded.ID != user.ID {
			t.Errorf("the account handed on for the role %s has id %v, want %v", account.role, seeded.ID, user.ID)
		}
	}
}

// TestRunner_GenerateUsers_ReportsAnAccountItCannotCreate verifies that a
// collision on an atname stops the generator with the account named, rather than
// leaving that one account missing from an otherwise successful run.
//
// [Ja] TestRunner_GenerateUsers_ReportsAnAccountItCannotCreate は、atname の衝突が、
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
		t.Fatal("generateUsers() should fail when the atname is already taken, but it succeeded")
	}
	if !strings.Contains(err.Error(), roster.users[0].atname) {
		t.Errorf("generateUsers() error = %q, want it to name the account it could not create", err)
	}
}

// TestRunner_GenerateWithdrawal verifies that the account the posts are read
// without an author is withdrawn the way the application withdraws one — soft
// deleted, with the email and the atname it held released — and that the
// accounts the conversations are read with are left able to sign in.
//
// [Ja] TestRunner_GenerateWithdrawal は、投稿を作者抜きで読むためのアカウントが、
// アプリケーションが退会させるのと同じやり方で退会させられること (論理削除され、持って
// いた email と atname が解放されること)、そして会話を読むためのアカウントがサインイン
// できる状態で残されることを検証します。
func TestRunner_GenerateWithdrawal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	st := &state{roster: testRoster()}

	tx := beginTx(t, db)
	runner := newTestRunner(db)
	if err := runner.generateUsers(ctx, tx, st); err != nil {
		t.Fatalf("generateUsers() error = %v", err)
	}
	if err := runner.generateWithdrawal(ctx, tx, st); err != nil {
		t.Fatalf("generateWithdrawal() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit the transaction: %v", err)
	}

	withdrawn := st.users.user(roleWithdrawn)
	if withdrawn == nil {
		t.Fatal("no account was created for the withdrawing role")
	}

	// The row is read directly rather than through a lookup, which would leave
	// the withdrawn account out by design.
	//
	// [Ja] 行はルックアップではなく直接読みます。ルックアップは、退会済みアカウントを
	// 設計上除外するためです。
	var deletedAt *time.Time
	var email, atname string
	if err := db.Reader.QueryRowContext(ctx,
		"SELECT deleted_at, email, atname FROM users WHERE id = ?", int64(withdrawn.ID),
	).Scan(&deletedAt, &email, &atname); err != nil {
		t.Fatalf("failed to read the withdrawn account: %v", err)
	}

	if deletedAt == nil {
		t.Error("the withdrawn account has no deleted_at, want the moment it withdrew")
	}
	if want := model.AnonymizedEmail(withdrawn.ID); email != want {
		t.Errorf("the withdrawn account's email = %q, want %q", email, want)
	}
	if want := model.AnonymizedAtname(withdrawn.ID); atname != want {
		t.Errorf("the withdrawn account's atname = %q, want %q", atname, want)
	}

	userRepo := repository.NewUserRepository(db)
	for _, role := range []seedRole{roleStarter, roleReplier} {
		account := st.users.user(role)
		if account == nil {
			t.Fatalf("no account was created for the role %s", role)
		}

		user, err := userRepo.FindByID(ctx, account.ID)
		if err != nil {
			t.Fatalf("FindByID(%v) error = %v", account.ID, err)
		}
		if user == nil {
			t.Errorf("the account for the role %s no longer resolves, want it to stay able to sign in", role)
		}
	}
}

// TestSignInRoles verifies that the list a command writes its usage from holds
// the roles whose accounts a finished run leaves able to sign in, and only
// those. The withdrawn role is a generator role too, but naming it in a usage
// would invite an invocation whose account the completed seed has disabled.
//
// [Ja] TestSignInRoles は、コマンドが usage を組み立てる元にする一覧が、完了した実行が
// サインイン可能なまま残すアカウントの役割だけを持つことを検証します。withdrawn も生成器の
// 役割ですが、usage に挙げると、完了したシードが無効化したアカウントの指定を促すことに
// なります。
func TestSignInRoles(t *testing.T) {
	t.Parallel()

	roles := SignInRoles()

	if want := []string{string(roleStarter), string(roleReplier)}; !slices.Equal(roles, want) {
		t.Errorf("SignInRoles() = %v, want %v", roles, want)
	}

	// Every role offered has to be one the roster is required to hold, or the
	// usage would name a role no account answers to.
	//
	// [Ja] 挙げる役割はすべて、名簿が持つことを要求される役割である必要があります。
	// そうでなければ、usage が、どのアカウントも応じない役割を挙げることになります。
	for _, role := range roles {
		if !slices.Contains(allSeedRoles, seedRole(role)) {
			t.Errorf("SignInRoles() offers %q, want it to name a role the generators know", role)
		}
	}
}
