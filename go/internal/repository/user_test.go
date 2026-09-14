package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserRepo builds a UserRepository over a database the test owns, so a test
// that only needs the repository does not have to hold on to the database
// itself.
//
// [Ja] newUserRepo はテストが所有するデータベース上に UserRepository を作る。
// リポジトリだけが必要なテストがデータベース自体を抱えずに済むようにするためである。
func newUserRepo(t *testing.T) (*repository.UserRepository, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	return repository.NewUserRepository(db), context.Background()
}

// findUser reads the account back through the repository, failing the test when
// it is not there, for the tests whose subject is what a write left behind
// rather than the lookup itself.
//
// [Ja] findUserはリポジトリ経由でアカウントを読み戻し、存在しなければテストを失敗させる。
// ルックアップそのものではなく、書き込みが何を残したかを主題とするテストのためのものである。
func findUser(t *testing.T, ctx context.Context, repo *repository.UserRepository, id model.UserID) *model.User {
	t.Helper()

	user, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if user == nil {
		t.Fatalf("FindByID() = nil, want user (id=%v)", id)
	}
	return user
}

func TestUserRepository_Create(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	user, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "create@example.com",
		Atname:   "createuser",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if user.ID == 0 {
		t.Error("Create() user.ID は DB 採番で空でないはず")
	}
	if user.Email != "create@example.com" {
		t.Errorf("user.Email = %q, want %q", user.Email, "create@example.com")
	}
	if user.Atname != "createuser" {
		t.Errorf("user.Atname = %q, want %q", user.Atname, "createuser")
	}
	if user.Locale != "ja" {
		t.Errorf("user.Locale = %q, want %q", user.Locale, "ja")
	}
	if user.TimeZone != "Asia/Tokyo" {
		t.Errorf("user.TimeZone = %q, want %q", user.TimeZone, "Asia/Tokyo")
	}
	if user.CreatedAt.IsZero() {
		t.Error("user.CreatedAt は DB 既定値で設定されるはず")
	}
	if user.UpdatedAt.IsZero() {
		t.Error("user.UpdatedAt は DB 既定値で設定されるはず")
	}
}

func TestUserRepository_FindByID(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	t.Run("存在するユーザーを取得できる", func(t *testing.T) {
		created, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "findbyid@example.com",
			Atname:   "findbyiduser",
			Locale:   "en",
			TimeZone: "UTC",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		user, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByID() = nil, want user")
		}
		if user.ID != created.ID {
			t.Errorf("user.ID = %v, want %v", user.ID, created.ID)
		}
		if user.Email != "findbyid@example.com" {
			t.Errorf("user.Email = %q, want %q", user.Email, "findbyid@example.com")
		}
	})

	t.Run("存在しないユーザーは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindByID(ctx, model.UserID(testutil.UnusedID))
		if err != nil {
			t.Fatalf("FindByID() error = %v, want nil", err)
		}
		if user != nil {
			t.Errorf("FindByID() = %v, want nil", user)
		}
	})
}

// TestUserRepository_Suspend verifies the two writes a suspension is made of:
// the mark goes on and comes back off, and the account's identity is untouched
// either way.
//
// [Ja] TestUserRepository_Suspendは停止を成す2つの書き込みを検証する。印が付き、そして
// 外れること、そしてそのどちらでもアカウントの身元が変わらないことである。
func TestUserRepository_Suspend(t *testing.T) {
	t.Parallel()

	t.Run("停止の時刻を立て、解除がそれを外す", func(t *testing.T) {
		t.Parallel()

		repo, ctx := newUserRepo(t)
		created, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "suspended@example.com",
			Atname:   "suspendeduser",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Suspend(ctx, created.ID); err != nil {
			t.Fatalf("Suspend() error = %v", err)
		}

		suspended := findUser(t, ctx, repo, created.ID)
		if suspended.SuspendedAt == nil {
			t.Fatal("user.SuspendedAt = nil, want the stamped time")
		}
		if suspended.SuspendedAt.Before(created.CreatedAt) {
			t.Errorf("user.SuspendedAt = %v, want at or after the account's creation (%v)", suspended.SuspendedAt, created.CreatedAt)
		}

		if err := repo.Unsuspend(ctx, created.ID); err != nil {
			t.Fatalf("Unsuspend() error = %v", err)
		}

		unsuspended := findUser(t, ctx, repo, created.ID)
		if unsuspended.SuspendedAt != nil {
			t.Errorf("user.SuspendedAt = %v, want nil", unsuspended.SuspendedAt)
		}
	})

	// A suspension stops what an account may do, not who it is, so the two
	// values that name it are what the write must leave alone.
	//
	// [Ja] 停止が止めるのはアカウントが何をできるかであって、それが誰であるかではない。
	// したがって、その身元を名指す2つの値こそが、この書き込みが触れてはならないものである。
	t.Run("停止は身元を変えない", func(t *testing.T) {
		t.Parallel()

		repo, ctx := newUserRepo(t)
		created, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "identity@example.com",
			Atname:   "identityuser",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Suspend(ctx, created.ID); err != nil {
			t.Fatalf("Suspend() error = %v", err)
		}

		user := findUser(t, ctx, repo, created.ID)
		if user.Email != "identity@example.com" {
			t.Errorf("user.Email = %q, want %q", user.Email, "identity@example.com")
		}
		if user.Atname != "identityuser" {
			t.Errorf("user.Atname = %q, want %q", user.Atname, "identityuser")
		}
		if user.DeletedAt != nil {
			t.Errorf("user.DeletedAt = %v, want nil (停止は退会ではない)", user.DeletedAt)
		}
	})
}

// TestUserRepository_SuspendedUsersAreExcludedFromTheSessionLookup verifies
// where a suspension takes effect and where it does not. The session lookup is
// the one place the mark hides the account, since resolving it there is what
// would let a cookie issued before the suspension go on acting; every other
// lookup still returns the row, because the admin screens that lift the
// suspension have to reach the account they are about.
//
// [Ja] TestUserRepository_SuspendedUsersAreExcludedFromTheSessionLookupは、停止が
// どこで効き、どこで効かないかを検証する。印がアカウントを隠すのはセッションの解決だけで
// ある。そこで解決されることこそが、停止の前に発行されたCookieを行動させ続けるもので
// あるためだ。ほかのルックアップは行を返し続ける。停止を解除する管理画面は、その対象の
// アカウントへ届かなければならないためである。
func TestUserRepository_SuspendedUsersAreExcludedFromTheSessionLookup(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// A fixed time stands in for the moment the administrator suspended the
	// account. It is written out rather than derived from time.Now, because the
	// column holds milliseconds and a value read back has to be comparable to
	// the one the test wrote.
	//
	// [Ja] 固定の時刻を、管理者がアカウントを停止した時点の代わりに使う。time.Nowから
	// 導かずに書き下すのは、列がミリ秒までを保持するためであり、読み戻した値が、テストが
	// 書いた値と比べられる必要があるためである。
	suspendedAt := time.Date(2026, 9, 12, 12, 30, 0, 0, time.UTC)

	t.Run("FindBySessionToken は停止中のアカウントのセッションを解決しない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).WithSuspendedAt(suspendedAt).Build()
		testutil.NewUserSessionBuilder(t, db).
			WithUserID(userID).
			WithToken("suspended-token").
			Build()

		user, err := repo.FindBySessionToken(ctx, "suspended-token")
		if err != nil {
			t.Fatalf("FindBySessionToken() error = %v", err)
		}
		if user != nil {
			t.Errorf("FindBySessionToken() = %v, want nil (停止中は解決されないはず)", user)
		}
	})

	t.Run("FindByID は停止中のアカウントを返し、停止の時刻を伝える", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).WithSuspendedAt(suspendedAt).Build()

		user, err := repo.FindByID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByID() = nil, want user (停止中でも管理画面から届くはず)")
		}
		if user.SuspendedAt == nil {
			t.Fatal("user.SuspendedAt = nil, want the stored timestamp")
		}
		if !user.SuspendedAt.Equal(suspendedAt) {
			t.Errorf("user.SuspendedAt = %v, want %v", user.SuspendedAt, suspendedAt)
		}
	})

	t.Run("FindByEmail は停止中のアカウントを返す", func(t *testing.T) {
		email := "suspended-findbyemail@example.com"
		testutil.NewUserBuilder(t, db).WithEmail(email).WithSuspendedAt(suspendedAt).Build()

		user, err := repo.FindByEmail(ctx, email)
		if err != nil {
			t.Fatalf("FindByEmail() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByEmail() = nil, want user (サインインの照合が停止を見分けるため)")
		}
		if user.SuspendedAt == nil {
			t.Error("user.SuspendedAt = nil, want the stored timestamp")
		}
	})
}
func TestUserRepository_FindByEmail(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "findbyemail@example.com",
		Atname:   "findbyemailuser",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("メールアドレスでユーザーを取得できる", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, "findbyemail@example.com")
		if err != nil {
			t.Fatalf("FindByEmail() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByEmail() = nil, want user")
		}
		if user.Email != "findbyemail@example.com" {
			t.Errorf("user.Email = %q, want %q", user.Email, "findbyemail@example.com")
		}
	})

	t.Run("NOCASE 照合により大文字小文字を無視して取得できる", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, "FindByEmail@Example.com")
		if err != nil {
			t.Fatalf("FindByEmail() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByEmail() = nil, want user (NOCASE 照合は大文字小文字を無視するはず)")
		}
	})

	t.Run("存在しないメールアドレスは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, "missing@example.com")
		if err != nil {
			t.Fatalf("FindByEmail() error = %v, want nil", err)
		}
		if user != nil {
			t.Errorf("FindByEmail() = %v, want nil", user)
		}
	})
}

// TestUserRepository_ListByIDs verifies the bulk lookup a thread's page resolves
// its authors through: the accounts still there come back in one query, the ids
// it is handed twice yield one row, an id naming nobody is simply absent, and a
// withdrawn account is left out the way it is left out of every other lookup
// here — which is what lets the page tell a withdrawn author from a present one
// by whether an id resolved at all.
//
// [Ja] TestUserRepository_ListByIDs は、スレッドのページが作者を解決するときの一括
// ルックアップを検証する。まだ存在するアカウントが 1 クエリで返ること、2 度渡された id が
// 1 行になること、誰も指さない id が単に含まれないこと、そして退会済みのアカウントが、
// ここの他のルックアップと同じく除外されることである。最後の点により、ページは id が
// そもそも解決したかどうかで、退会した作者と現存する作者を区別できる。
func TestUserRepository_ListByIDs(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	alice := testutil.NewUserBuilder(t, db).WithAtname("listalice").WithEmail("listalice@example.com").Build()
	bob := testutil.NewUserBuilder(t, db).WithAtname("listbob").WithEmail("listbob@example.com").Build()
	withdrawn := testutil.NewUserBuilder(t, db).
		WithAtname("listwithdrawn").
		WithEmail("listwithdrawn@example.com").
		WithDeletedAt(time.Now().Add(-24 * time.Hour)).
		Build()

	t.Run("渡した id のアカウントをまとめて返す", func(t *testing.T) {
		users, err := repo.ListByIDs(ctx, []model.UserID{bob, alice, alice})
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("len(users) = %d, want 2", len(users))
		}

		atnames := map[model.UserID]string{}
		for _, user := range users {
			atnames[user.ID] = user.Atname
		}
		if atnames[alice] != "listalice" {
			t.Errorf("alice の atname = %q, want %q", atnames[alice], "listalice")
		}
		if atnames[bob] != "listbob" {
			t.Errorf("bob の atname = %q, want %q", atnames[bob], "listbob")
		}
	})

	t.Run("退会済みのアカウントと存在しない id は含まれない", func(t *testing.T) {
		users, err := repo.ListByIDs(ctx, []model.UserID{alice, withdrawn, alice + 100000})
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(users) != 1 {
			t.Fatalf("len(users) = %d, want 1", len(users))
		}
		if users[0].ID != alice {
			t.Errorf("users[0].ID = %v, want %v", users[0].ID, alice)
		}
	})

	t.Run("id が空なら空のスライスを返す", func(t *testing.T) {
		users, err := repo.ListByIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(users) != 0 {
			t.Errorf("len(users) = %d, want 0", len(users))
		}
	})
}

func TestUserRepository_FindBySessionToken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, db).
		WithEmail("session-user@example.com").
		Build()
	testutil.NewUserSessionBuilder(t, db).
		WithUserID(userID).
		WithToken("resolve-token").
		Build()

	t.Run("セッショントークンから所有ユーザーを解決できる", func(t *testing.T) {
		user, err := repo.FindBySessionToken(ctx, "resolve-token")
		if err != nil {
			t.Fatalf("FindBySessionToken() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindBySessionToken() = nil, want user")
		}
		if user.ID != userID {
			t.Errorf("user.ID = %v, want %v", user.ID, userID)
		}
		if user.Email != "session-user@example.com" {
			t.Errorf("user.Email = %q, want %q", user.Email, "session-user@example.com")
		}
	})

	t.Run("一致するセッションが無いトークンは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindBySessionToken(ctx, "no-such-token")
		if err != nil {
			t.Fatalf("FindBySessionToken() error = %v, want nil", err)
		}
		if user != nil {
			t.Errorf("FindBySessionToken() = %v, want nil", user)
		}
	})
}

// TestUserRepository_CreateRejectsDuplicateEmail verifies the users.email UNIQUE
// constraint surfaces as an error (case-insensitive via the NOCASE collation).
//
// [Ja] TestUserRepository_CreateRejectsDuplicateEmail は users.email の UNIQUE 制約が
// エラーとして表面化することを確認する (NOCASE 照合により大文字小文字を区別しない)。
func TestUserRepository_CreateRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	// The two rows carry distinct atnames so the second insert fails specifically
	// on the users.email UNIQUE constraint, not on users.atname.
	//
	// [Ja] 2 行は異なる atname を持たせ、2 回目の挿入が users.atname ではなく
	// users.email の UNIQUE 制約でこそ失敗するようにする。
	if _, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "dup@example.com",
		Atname:   "dupemailone",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "DUP@example.com",
		Atname:   "dupemailtwo",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err == nil {
		t.Error("重複メールアドレスの Create() はエラーになるはず")
	}
}

// TestUserRepository_FindByAtname verifies lookup by atname: an existing atname
// resolves the user, the match is case-insensitive via the NOCASE collation, and an unknown
// atname returns (nil, nil).
//
// [Ja] TestUserRepository_FindByAtname は atname による取得を検証する。存在する atname は
// ユーザーを解決し、照合は NOCASE 照合により大文字小文字を無視し、未知の atname は (nil, nil)
// を返す。
func TestUserRepository_FindByAtname(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "findbyatname@example.com",
		Atname:   "findbyatnameuser",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("atname でユーザーを取得できる", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, "findbyatnameuser")
		if err != nil {
			t.Fatalf("FindByAtname() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByAtname() = nil, want user")
		}
		if user.Atname != "findbyatnameuser" {
			t.Errorf("user.Atname = %q, want %q", user.Atname, "findbyatnameuser")
		}
	})

	t.Run("NOCASE 照合により大文字小文字を無視して取得できる", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, "FindByAtnameUser")
		if err != nil {
			t.Fatalf("FindByAtname() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByAtname() = nil, want user (NOCASE 照合は大文字小文字を無視するはず)")
		}
	})

	t.Run("存在しない atname は (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, "missingatname")
		if err != nil {
			t.Fatalf("FindByAtname() error = %v, want nil", err)
		}
		if user != nil {
			t.Errorf("FindByAtname() = %v, want nil", user)
		}
	})
}

// TestUserRepository_CreateRejectsDuplicateAtname verifies the users.atname UNIQUE
// constraint surfaces as an error (case-insensitive via the NOCASE collation).
//
// [Ja] TestUserRepository_CreateRejectsDuplicateAtname は users.atname の UNIQUE 制約が
// エラーとして表面化することを確認する (NOCASE 照合により大文字小文字を区別しない)。
func TestUserRepository_CreateRejectsDuplicateAtname(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "dupatname1@example.com",
		Atname:   "dupatname",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "dupatname2@example.com",
		Atname:   "DupAtname",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err == nil {
		t.Error("重複 atname の Create() はエラーになるはず")
	}
}

// TestUserRepository_SoftDeletedUsersAreExcludedFromLookups verifies that a user
// whose deleted_at is set (a withdrawn account) is resolved by none of the
// authentication lookups. This is the lookup-level defense that stops a withdrawn
// user from signing in or being resolved from a still-present session, on top of
// the withdrawal flow deleting the session rows.
//
// [Ja] TestUserRepository_SoftDeletedUsersAreExcludedFromLookups は deleted_at が
// セットされたユーザー (退会済みアカウント) が、いずれの認証系ルックアップでも解決され
// ないことを検証する。これは退会フローがセッション行を削除することに加えた、ルックアップ
// 層での防御であり、退会済みユーザーがサインインしたり、残存セッションから解決されたり
// するのを防ぐ。
func TestUserRepository_SoftDeletedUsersAreExcludedFromLookups(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// A time in the past stands in for the moment the user withdrew; the exact
	// value does not matter, only that deleted_at is non-null.
	//
	// [Ja] 過去の時刻をユーザーが退会した時点の代わりに使う。deleted_at が非 NULL である
	// ことだけが重要で、具体的な値は問わない。
	deletedAt := time.Now().Add(-24 * time.Hour)

	t.Run("FindByID は論理削除済みユーザーを除外する", func(t *testing.T) {
		id := testutil.NewUserBuilder(t, db).WithDeletedAt(deletedAt).Build()

		user, err := repo.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if user != nil {
			t.Errorf("FindByID() = %v, want nil (論理削除済みは除外されるはず)", user)
		}
	})

	t.Run("FindByEmail は論理削除済みユーザーを除外する", func(t *testing.T) {
		email := "softdeleted-findbyemail@example.com"
		testutil.NewUserBuilder(t, db).WithEmail(email).WithDeletedAt(deletedAt).Build()

		user, err := repo.FindByEmail(ctx, email)
		if err != nil {
			t.Fatalf("FindByEmail() error = %v", err)
		}
		if user != nil {
			t.Errorf("FindByEmail() = %v, want nil (論理削除済みは除外されるはず)", user)
		}
	})

	t.Run("FindByAtname は論理削除済みユーザーを除外する", func(t *testing.T) {
		atname := "softdeletedatname"
		testutil.NewUserBuilder(t, db).WithAtname(atname).WithDeletedAt(deletedAt).Build()

		user, err := repo.FindByAtname(ctx, atname)
		if err != nil {
			t.Fatalf("FindByAtname() error = %v", err)
		}
		if user != nil {
			t.Errorf("FindByAtname() = %v, want nil (論理削除済みは除外されるはず)", user)
		}
	})

	t.Run("FindBySessionToken は論理削除済みユーザーのセッションを解決しない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).WithDeletedAt(deletedAt).Build()
		testutil.NewUserSessionBuilder(t, db).
			WithUserID(userID).
			WithToken("soft-deleted-token").
			Build()

		user, err := repo.FindBySessionToken(ctx, "soft-deleted-token")
		if err != nil {
			t.Fatalf("FindBySessionToken() error = %v", err)
		}
		if user != nil {
			t.Errorf("FindBySessionToken() = %v, want nil (論理削除済みは除外されるはず)", user)
		}
	})
}

// TestUserRepository_SoftDeleteAndAnonymize verifies the withdrawal write: it
// stamps deleted_at and overwrites email and atname with the given anonymized
// values, the soft-deleted row drops out of the authentication lookups, and the
// original email and atname are freed for another account to reclaim.
//
// [Ja] TestUserRepository_SoftDeleteAndAnonymize は退会の書き込みを検証する。deleted_at を
// 打ち、email と atname を与えられた匿名値で上書きし、論理削除された行が認証系ルックアップ
// から外れ、元の email と atname が別アカウントの再取得のために解放されることを確かめる。
func TestUserRepository_SoftDeleteAndAnonymize(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	created, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "withdraw-me@example.com",
		Atname:   "withdrawme",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	anonEmail := "deleted-" + created.ID.String() + "@deleted.invalid"
	anonAtname := "deleted-" + created.ID.String()

	if err := repo.SoftDeleteAndAnonymize(ctx, created.ID, anonEmail, anonAtname); err != nil {
		t.Fatalf("SoftDeleteAndAnonymize() error = %v", err)
	}

	t.Run("deleted_at がセットされ email/atname が匿名値になる", func(t *testing.T) {
		// The row is queried directly (not via FindByID, which filters deleted_at)
		// so the soft-deleted, anonymized values are observable.
		//
		// [Ja] 行は (deleted_at で絞る FindByID ではなく) 直接クエリするため、論理削除・
		// 匿名化された値を観測できる。
		var deletedAt *time.Time
		var email, atname string
		if err := db.Writer.QueryRowContext(ctx,
			`SELECT deleted_at, email, atname FROM users WHERE id = ?`, int64(created.ID),
		).Scan(&deletedAt, &email, &atname); err != nil {
			t.Fatalf("行の取得に失敗: %v", err)
		}
		if deletedAt == nil {
			t.Error("deleted_at がセットされていない")
		}
		if email != anonEmail {
			t.Errorf("email = %q, want %q", email, anonEmail)
		}
		if atname != anonAtname {
			t.Errorf("atname = %q, want %q", atname, anonAtname)
		}
	})

	t.Run("論理削除後は FindByID から外れる", func(t *testing.T) {
		user, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if user != nil {
			t.Error("論理削除後の FindByID() は nil を返すはず")
		}
	})

	t.Run("解放された email と atname は別アカウントが再取得できる", func(t *testing.T) {
		if _, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "withdraw-me@example.com",
			Atname:   "withdrawme",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		}); err != nil {
			t.Errorf("解放された email/atname での Create() error = %v, want nil (再取得できるはず)", err)
		}
	})
}

// TestUserRepository_UpdateEmail verifies UpdateEmail rewrites the user's email
// and that moving to an address already taken by another account fails on the
// users.email UNIQUE constraint (case-insensitive via the NOCASE collation).
//
// [Ja] TestUserRepository_UpdateEmail は UpdateEmail がユーザーの email を書き換えること、
// および別アカウントが既に使用しているアドレスへの変更が users.email の UNIQUE 制約で失敗
// することを検証する (NOCASE 照合により大文字小文字を区別しない)。
func TestUserRepository_UpdateEmail(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	t.Run("メールアドレスを更新できる", func(t *testing.T) {
		created, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "before@example.com",
			Atname:   "updateemailuser",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.UpdateEmail(ctx, created.ID, "after@example.com"); err != nil {
			t.Fatalf("UpdateEmail() error = %v", err)
		}

		user, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByID() = nil, want user")
		}
		if user.Email != "after@example.com" {
			t.Errorf("user.Email = %q, want %q", user.Email, "after@example.com")
		}
	})

	t.Run("既存アカウントと重複するアドレスへの更新はエラー", func(t *testing.T) {
		// Another account already holds taken@example.com, so moving to it fails
		// on the users.email UNIQUE constraint (NOCASE, case-insensitive).
		//
		// [Ja] 別アカウントが taken@example.com を先に使用しているため、そのアドレスへの
		// 更新は users.email の UNIQUE 制約 (NOCASE 照合) で失敗する。
		if _, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "taken@example.com",
			Atname:   "takenemailuser",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		}); err != nil {
			t.Fatalf("既存ユーザーの Create() error = %v", err)
		}
		mover, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "mover@example.com",
			Atname:   "moveremailuser",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.UpdateEmail(ctx, mover.ID, "Taken@Example.com"); err == nil {
			t.Error("重複アドレスへの UpdateEmail() はエラーになるはず")
		}
	})
}

// listAtnames returns the atnames of the users in the order they were listed,
// so a test states the page it expects as the handles it would read on it.
//
// [Ja] listAtnames は、一覧に並んだユーザーの atname をその順序のまま返す。テストが
// 期待するページを、その画面で読むことになるハンドルの並びとして書けるようにするため
// である。
func listAtnames(users []*model.User) []string {
	atnames := make([]string, len(users))
	for i, user := range users {
		atnames[i] = user.Atname
	}
	return atnames
}

// assertAtnames fails the test unless the listing holds exactly the given
// atnames in the given order.
//
// [Ja] assertAtnames は、一覧が指定した atname を指定した順序でちょうど持たない限り
// テストを失敗させる。
func assertAtnames(t *testing.T, users []*model.User, want []string) {
	t.Helper()

	got := listAtnames(users)
	if len(got) != len(want) {
		t.Fatalf("一覧の atname = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("一覧の atname = %v, want %v", got, want)
		}
	}
}

func TestUserRepository_ListPageByAtnamePrefix(t *testing.T) {
	t.Parallel()

	t.Run("空の prefix は退会していない利用者を登録の新しい順に返す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("first").Build()
		testutil.NewUserBuilder(t, db).WithAtname("second").Build()
		testutil.NewUserBuilder(t, db).WithAtname("third").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}

		assertAtnames(t, users, []string{"third", "second", "first"})
	})

	t.Run("退会した利用者を含めない", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("staying").Build()
		testutil.NewUserBuilder(t, db).WithAtname("leaving").WithDeletedAt(time.Now()).Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}

		assertAtnames(t, users, []string{"staying"})
	})

	t.Run("limit と offset が 1 ページ分ずつを切り出す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("older").Build()
		testutil.NewUserBuilder(t, db).WithAtname("middle").Build()
		testutil.NewUserBuilder(t, db).WithAtname("newer").Build()

		first, err := repo.ListPageByAtnamePrefix(ctx, "", 2, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}
		assertAtnames(t, first, []string{"newer", "middle"})

		second, err := repo.ListPageByAtnamePrefix(ctx, "", 2, 2)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}
		assertAtnames(t, second, []string{"older"})
	})

	t.Run("最後のページを越えた offset は空を返す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("only").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "", 10, 50)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}

		assertAtnames(t, users, nil)
	})

	t.Run("prefix は前方一致で、大文字小文字を区別しない", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("Alice").Build()
		testutil.NewUserBuilder(t, db).WithAtname("alberta").Build()
		testutil.NewUserBuilder(t, db).WithAtname("bob").Build()
		testutil.NewUserBuilder(t, db).WithAtname("carolal").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "AL", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}

		assertAtnames(t, users, []string{"alberta", "Alice"})
	})

	t.Run("prefix そのものと一致する atname も含める", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("ada").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "ada", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}

		assertAtnames(t, users, []string{"ada"})
	})

	// The underscore is part of the atname character set, so a search carrying
	// one addresses that character. Were the match written as LIKE, it would be
	// LIKE's single-character wildcard instead and the search would also reach
	// the accounts spelling any other character there.
	//
	// [Ja] アンダースコアは atname の文字集合の一部であるため、それを含む検索はその文字を
	// 名指している。一致を LIKE で書けばこれは LIKE の 1 文字ワイルドカードになり、検索は
	// その位置に別の文字を綴るアカウントにも届いてしまう。
	t.Run("アンダースコアはワイルドカードではなく文字として一致する", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("a_b").Build()
		testutil.NewUserBuilder(t, db).WithAtname("axb").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "a_", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}

		assertAtnames(t, users, []string{"a_b"})
	})

	t.Run("prefix に一致する利用者がいなければ空を返す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("ada").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "zoe", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix() error = %v", err)
		}

		assertAtnames(t, users, nil)
	})
}

func TestUserRepository_CountByAtnamePrefix(t *testing.T) {
	t.Parallel()

	t.Run("空の prefix は退会していない利用者を数える", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("staying").Build()
		testutil.NewUserBuilder(t, db).WithAtname("alsostaying").Build()
		testutil.NewUserBuilder(t, db).WithAtname("leaving").WithDeletedAt(time.Now()).Build()

		count, err := repo.CountByAtnamePrefix(ctx, "")
		if err != nil {
			t.Fatalf("CountByAtnamePrefix() error = %v", err)
		}
		if count != 2 {
			t.Errorf("CountByAtnamePrefix(\"\") = %d, want 2", count)
		}
	})

	t.Run("prefix で絞り込んだ件数を大文字小文字を区別せずに数える", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("Alice").Build()
		testutil.NewUserBuilder(t, db).WithAtname("alberta").Build()
		testutil.NewUserBuilder(t, db).WithAtname("bob").Build()
		testutil.NewUserBuilder(t, db).WithAtname("albert").WithDeletedAt(time.Now()).Build()

		count, err := repo.CountByAtnamePrefix(ctx, "AL")
		if err != nil {
			t.Fatalf("CountByAtnamePrefix() error = %v", err)
		}
		if count != 2 {
			t.Errorf("CountByAtnamePrefix(\"AL\") = %d, want 2", count)
		}
	})
}

// TestUserRepository_ListPageByAtnamePrefix_UsesTheAtnameIndex verifies that the
// narrowed listing is answered through the index behind the atname UNIQUE
// constraint. The search is what a page of the admin screens is filtered with,
// and reading every account to answer it would make the screen slower the more
// people the community has.
//
// [Ja] TestUserRepository_ListPageByAtnamePrefix_UsesTheAtnameIndex は、絞り込んだ一覧が
// atname の UNIQUE 制約を支える索引で答えられることを検証します。この検索は管理画面の
// ページを絞り込むものであり、答えるためにすべてのアカウントを読めば、その画面は
// コミュニティの人数が増えるほど遅くなります。
func TestUserRepository_ListPageByAtnamePrefix_UsesTheAtnameIndex(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := context.Background()

	statement := queryStatement(t, "users.sql", "ListUsersPageByAtnamePrefix")
	plan := queryPlan(t, ctx, db, statement, "a", "a\U0010FFFF", int64(50), int64(0))

	// SQLite names the index enforcing a UNIQUE constraint it was not given a
	// name for after the table it stands on, users being the second such
	// constraint's owner here (email is the first).
	//
	// [Ja] SQLite は、名前を与えられていない UNIQUE 制約を支える索引を、その制約が立つ
	// テーブルの名前から名付ける。ここで users の 2 つ目の制約にあたるのが atname で
	// ある (1 つ目は email)。
	const index = "sqlite_autoindex_users_2"
	if !strings.Contains(plan, index) {
		t.Errorf("絞り込んだ一覧の実行計画 = %q, %q をたどるはず", plan, index)
	}
	if strings.Contains(plan, "SCAN users") {
		t.Errorf("絞り込んだ一覧の実行計画 = %q, users の全走査を伴わないはず", plan)
	}
}
