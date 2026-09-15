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

// newUserRepoはテストが所有するデータベース上にUserRepositoryを作る。
// リポジトリだけが必要なテストがデータベース自体を抱えずに済むようにするためである。
func newUserRepo(t *testing.T) (*repository.UserRepository, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	return repository.NewUserRepository(db), context.Background()
}

// findUserはリポジトリ経由でアカウントを読み戻し、存在しなければテストを失敗させる。
// ルックアップそのものではなく、書き込みが何を残したかを主題とするテストのためのものである。
func findUser(t *testing.T, ctx context.Context, repo *repository.UserRepository, id model.UserID) *model.User {
	t.Helper()

	user, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if user == nil {
		t.Fatalf("FindByID() = nil、期待値はユーザー (id=%v)", id)
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	if user.ID == 0 {
		t.Error("Create() user.IDはDB採番で空でないはず")
	}
	if user.Email != "create@example.com" {
		t.Errorf("user.Email = %q、期待値 = %q", user.Email, "create@example.com")
	}
	if user.Atname != "createuser" {
		t.Errorf("user.Atname = %q、期待値 = %q", user.Atname, "createuser")
	}
	if user.Locale != "ja" {
		t.Errorf("user.Locale = %q、期待値 = %q", user.Locale, "ja")
	}
	if user.TimeZone != "Asia/Tokyo" {
		t.Errorf("user.TimeZone = %q、期待値 = %q", user.TimeZone, "Asia/Tokyo")
	}
	if user.CreatedAt.IsZero() {
		t.Error("user.CreatedAtはDB既定値で設定されるはず")
	}
	if user.UpdatedAt.IsZero() {
		t.Error("user.UpdatedAtはDB既定値で設定されるはず")
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
			t.Fatalf("Create()のエラー = %v", err)
		}

		user, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByID() = nil、期待値はユーザー")
		}
		if user.ID != created.ID {
			t.Errorf("user.ID = %v、期待値 = %v", user.ID, created.ID)
		}
		if user.Email != "findbyid@example.com" {
			t.Errorf("user.Email = %q、期待値 = %q", user.Email, "findbyid@example.com")
		}
	})

	t.Run("存在しないユーザーは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindByID(ctx, model.UserID(testutil.UnusedID))
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v、期待値 = nil", err)
		}
		if user != nil {
			t.Errorf("FindByID() = %v、期待値 = nil", user)
		}
	})
}

// TestUserRepository_Suspendは停止を成す2つの書き込みを検証する。印が付き、そして
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
			t.Fatalf("Create()のエラー = %v", err)
		}

		if err := repo.Suspend(ctx, created.ID); err != nil {
			t.Fatalf("Suspend()のエラー = %v", err)
		}

		suspended := findUser(t, ctx, repo, created.ID)
		if suspended.SuspendedAt == nil {
			t.Fatal("user.SuspendedAt = nil、期待値は打刻された時刻")
		}
		if suspended.SuspendedAt.Before(created.CreatedAt) {
			t.Errorf("user.SuspendedAt = %v、期待値はアカウントの作成時刻 (%v) 以降", suspended.SuspendedAt, created.CreatedAt)
		}

		if err := repo.Unsuspend(ctx, created.ID); err != nil {
			t.Fatalf("Unsuspend()のエラー = %v", err)
		}

		unsuspended := findUser(t, ctx, repo, created.ID)
		if unsuspended.SuspendedAt != nil {
			t.Errorf("user.SuspendedAt = %v、期待値 = nil", unsuspended.SuspendedAt)
		}
	})

	// 停止が止めるのはアカウントが何をできるかであって、それが誰であるかではない。
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
			t.Fatalf("Create()のエラー = %v", err)
		}

		if err := repo.Suspend(ctx, created.ID); err != nil {
			t.Fatalf("Suspend()のエラー = %v", err)
		}

		user := findUser(t, ctx, repo, created.ID)
		if user.Email != "identity@example.com" {
			t.Errorf("user.Email = %q、期待値 = %q", user.Email, "identity@example.com")
		}
		if user.Atname != "identityuser" {
			t.Errorf("user.Atname = %q、期待値 = %q", user.Atname, "identityuser")
		}
		if user.DeletedAt != nil {
			t.Errorf("user.DeletedAt = %v、期待値 = nil (停止は退会ではない)", user.DeletedAt)
		}
	})
}

// TestUserRepository_SuspendedUsersAreExcludedFromTheSessionLookupは、停止が
// どこで効き、どこで効かないかを検証する。印がアカウントを隠すのはセッションの解決だけで
// ある。そこで解決されることこそが、停止の前に発行されたCookieを行動させ続けるもので
// あるためだ。ほかのルックアップは行を返し続ける。停止を解除する管理画面は、その対象の
// アカウントへ届かなければならないためである。
func TestUserRepository_SuspendedUsersAreExcludedFromTheSessionLookup(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// 固定の時刻を、管理者がアカウントを停止した時点の代わりに使う。time.Nowから
	// 導かずに書き下すのは、列がミリ秒までを保持するためであり、読み戻した値が、テストが
	// 書いた値と比べられる必要があるためである。
	suspendedAt := time.Date(2026, 9, 12, 12, 30, 0, 0, time.UTC)

	t.Run("FindBySessionTokenは停止中のアカウントのセッションを解決しない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).WithSuspendedAt(suspendedAt).Build()
		testutil.NewUserSessionBuilder(t, db).
			WithUserID(userID).
			WithToken("suspended-token").
			Build()

		user, err := repo.FindBySessionToken(ctx, "suspended-token")
		if err != nil {
			t.Fatalf("FindBySessionToken()のエラー = %v", err)
		}
		if user != nil {
			t.Errorf("FindBySessionToken() = %v、期待値 = nil (停止中は解決されないはず)", user)
		}
	})

	t.Run("FindByIDは停止中のアカウントを返し、停止の時刻を伝える", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).WithSuspendedAt(suspendedAt).Build()

		user, err := repo.FindByID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByID() = nil、期待値はユーザー (停止中でも管理画面から届くはず)")
		}
		if user.SuspendedAt == nil {
			t.Fatal("user.SuspendedAt = nil、期待値は保存された時刻")
		}
		if !user.SuspendedAt.Equal(suspendedAt) {
			t.Errorf("user.SuspendedAt = %v、期待値 = %v", user.SuspendedAt, suspendedAt)
		}
	})

	t.Run("FindByEmailは停止中のアカウントを返す", func(t *testing.T) {
		email := "suspended-findbyemail@example.com"
		testutil.NewUserBuilder(t, db).WithEmail(email).WithSuspendedAt(suspendedAt).Build()

		user, err := repo.FindByEmail(ctx, email)
		if err != nil {
			t.Fatalf("FindByEmail()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByEmail() = nil、期待値はユーザー (サインインの照合が停止を見分けるため)")
		}
		if user.SuspendedAt == nil {
			t.Error("user.SuspendedAt = nil、期待値は保存された時刻")
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("メールアドレスでユーザーを取得できる", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, "findbyemail@example.com")
		if err != nil {
			t.Fatalf("FindByEmail()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByEmail() = nil、期待値はユーザー")
		}
		if user.Email != "findbyemail@example.com" {
			t.Errorf("user.Email = %q、期待値 = %q", user.Email, "findbyemail@example.com")
		}
	})

	t.Run("NOCASE照合により大文字小文字を無視して取得できる", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, "FindByEmail@Example.com")
		if err != nil {
			t.Fatalf("FindByEmail()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByEmail() = nil、期待値はユーザー (NOCASE照合は大文字小文字を無視するはず)")
		}
	})

	t.Run("存在しないメールアドレスは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, "missing@example.com")
		if err != nil {
			t.Fatalf("FindByEmail()のエラー = %v、期待値 = nil", err)
		}
		if user != nil {
			t.Errorf("FindByEmail() = %v、期待値 = nil", user)
		}
	})
}

// TestUserRepository_ListByIDsは、スレッドのページが作者を解決するときの一括
// ルックアップを検証する。まだ存在するアカウントが1クエリで返ること、2度渡されたidが
// 1行になること、誰も指さないidが単に含まれないこと、そして退会済みのアカウントが、
// ここの他のルックアップと同じく除外されることである。最後の点により、ページはidが
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

	t.Run("渡したidのアカウントをまとめて返す", func(t *testing.T) {
		users, err := repo.ListByIDs(ctx, []model.UserID{bob, alice, alice})
		if err != nil {
			t.Fatalf("ListByIDs()のエラー = %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("len(users) = %d、期待値 = 2", len(users))
		}

		atnames := map[model.UserID]string{}
		for _, user := range users {
			atnames[user.ID] = user.Atname
		}
		if atnames[alice] != "listalice" {
			t.Errorf("aliceのatname = %q、期待値 = %q", atnames[alice], "listalice")
		}
		if atnames[bob] != "listbob" {
			t.Errorf("bobのatname = %q、期待値 = %q", atnames[bob], "listbob")
		}
	})

	t.Run("退会済みのアカウントと存在しないidは含まれない", func(t *testing.T) {
		users, err := repo.ListByIDs(ctx, []model.UserID{alice, withdrawn, alice + 100000})
		if err != nil {
			t.Fatalf("ListByIDs()のエラー = %v", err)
		}
		if len(users) != 1 {
			t.Fatalf("len(users) = %d、期待値 = 1", len(users))
		}
		if users[0].ID != alice {
			t.Errorf("users[0].ID = %v、期待値 = %v", users[0].ID, alice)
		}
	})

	t.Run("idが空なら空のスライスを返す", func(t *testing.T) {
		users, err := repo.ListByIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByIDs()のエラー = %v", err)
		}
		if len(users) != 0 {
			t.Errorf("len(users) = %d、期待値 = 0", len(users))
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
			t.Fatalf("FindBySessionToken()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindBySessionToken() = nil、期待値はユーザー")
		}
		if user.ID != userID {
			t.Errorf("user.ID = %v、期待値 = %v", user.ID, userID)
		}
		if user.Email != "session-user@example.com" {
			t.Errorf("user.Email = %q、期待値 = %q", user.Email, "session-user@example.com")
		}
	})

	t.Run("一致するセッションが無いトークンは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindBySessionToken(ctx, "no-such-token")
		if err != nil {
			t.Fatalf("FindBySessionToken()のエラー = %v、期待値 = nil", err)
		}
		if user != nil {
			t.Errorf("FindBySessionToken() = %v、期待値 = nil", user)
		}
	})
}

// TestUserRepository_CreateRejectsDuplicateEmailはusers.emailのUNIQUE制約が
// エラーとして表面化することを確認する (NOCASE照合により大文字小文字を区別しない)。
func TestUserRepository_CreateRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	// 2行は異なるatnameを持たせ、2回目の挿入がusers.atnameではなく
	// users.emailのUNIQUE制約でこそ失敗するようにする。
	if _, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "dup@example.com",
		Atname:   "dupemailone",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	}); err != nil {
		t.Fatalf("1回目のCreate()のエラー = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "DUP@example.com",
		Atname:   "dupemailtwo",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err == nil {
		t.Error("重複メールアドレスのCreate() はエラーになるはず")
	}
}

// TestUserRepository_FindByAtnameはatnameによる取得を検証する。存在するatnameは
// ユーザーを解決し、照合はNOCASE照合により大文字小文字を無視し、未知のatnameは (nil, nil)
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("atnameでユーザーを取得できる", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, "findbyatnameuser")
		if err != nil {
			t.Fatalf("FindByAtname()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByAtname() = nil、期待値はユーザー")
		}
		if user.Atname != "findbyatnameuser" {
			t.Errorf("user.Atname = %q、期待値 = %q", user.Atname, "findbyatnameuser")
		}
	})

	t.Run("NOCASE照合により大文字小文字を無視して取得できる", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, "FindByAtnameUser")
		if err != nil {
			t.Fatalf("FindByAtname()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByAtname() = nil、期待値はユーザー (NOCASE照合は大文字小文字を無視するはず)")
		}
	})

	t.Run("存在しないatnameは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, "missingatname")
		if err != nil {
			t.Fatalf("FindByAtname()のエラー = %v、期待値 = nil", err)
		}
		if user != nil {
			t.Errorf("FindByAtname() = %v、期待値 = nil", user)
		}
	})
}

// TestUserRepository_CreateRejectsDuplicateAtnameはusers.atnameのUNIQUE制約が
// エラーとして表面化することを確認する (NOCASE照合により大文字小文字を区別しない)。
func TestUserRepository_CreateRejectsDuplicateAtname(t *testing.T) {
	t.Parallel()

	repo, ctx := newUserRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "dupatname1@example.com",
		Atname:   "dupatname",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	}); err != nil {
		t.Fatalf("1回目のCreate()のエラー = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    "dupatname2@example.com",
		Atname:   "DupAtname",
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err == nil {
		t.Error("重複atnameのCreate() はエラーになるはず")
	}
}

// TestUserRepository_SoftDeletedUsersAreExcludedFromLookupsはdeleted_atが
// セットされたユーザー (退会済みアカウント) が、いずれの認証系ルックアップでも解決され
// ないことを検証する。これは退会フローがセッション行を削除することに加えた、ルックアップ
// 層での防御であり、退会済みユーザーがサインインしたり、残存セッションから解決されたり
// するのを防ぐ。
func TestUserRepository_SoftDeletedUsersAreExcludedFromLookups(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// 過去の時刻をユーザーが退会した時点の代わりに使う。deleted_atが非NULLである
	// ことだけが重要で、具体的な値は問わない。
	deletedAt := time.Now().Add(-24 * time.Hour)

	t.Run("FindByIDは論理削除済みユーザーを除外する", func(t *testing.T) {
		id := testutil.NewUserBuilder(t, db).WithDeletedAt(deletedAt).Build()

		user, err := repo.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if user != nil {
			t.Errorf("FindByID() = %v、期待値 = nil (論理削除済みは除外されるはず)", user)
		}
	})

	t.Run("FindByEmailは論理削除済みユーザーを除外する", func(t *testing.T) {
		email := "softdeleted-findbyemail@example.com"
		testutil.NewUserBuilder(t, db).WithEmail(email).WithDeletedAt(deletedAt).Build()

		user, err := repo.FindByEmail(ctx, email)
		if err != nil {
			t.Fatalf("FindByEmail()のエラー = %v", err)
		}
		if user != nil {
			t.Errorf("FindByEmail() = %v、期待値 = nil (論理削除済みは除外されるはず)", user)
		}
	})

	t.Run("FindByAtnameは論理削除済みユーザーを除外する", func(t *testing.T) {
		atname := "softdeletedatname"
		testutil.NewUserBuilder(t, db).WithAtname(atname).WithDeletedAt(deletedAt).Build()

		user, err := repo.FindByAtname(ctx, atname)
		if err != nil {
			t.Fatalf("FindByAtname()のエラー = %v", err)
		}
		if user != nil {
			t.Errorf("FindByAtname() = %v、期待値 = nil (論理削除済みは除外されるはず)", user)
		}
	})

	t.Run("FindBySessionTokenは論理削除済みユーザーのセッションを解決しない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).WithDeletedAt(deletedAt).Build()
		testutil.NewUserSessionBuilder(t, db).
			WithUserID(userID).
			WithToken("soft-deleted-token").
			Build()

		user, err := repo.FindBySessionToken(ctx, "soft-deleted-token")
		if err != nil {
			t.Fatalf("FindBySessionToken()のエラー = %v", err)
		}
		if user != nil {
			t.Errorf("FindBySessionToken() = %v、期待値 = nil (論理削除済みは除外されるはず)", user)
		}
	})
}

// TestUserRepository_SoftDeleteAndAnonymizeは退会の書き込みを検証する。deleted_atを
// 打ち、emailとatnameを与えられた匿名値で上書きし、論理削除された行が認証系ルックアップ
// から外れ、元のemailとatnameが別アカウントの再取得のために解放されることを確かめる。
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	anonEmail := "deleted-" + created.ID.String() + "@deleted.invalid"
	anonAtname := "deleted-" + created.ID.String()

	if err := repo.SoftDeleteAndAnonymize(ctx, created.ID, anonEmail, anonAtname); err != nil {
		t.Fatalf("SoftDeleteAndAnonymize()のエラー = %v", err)
	}

	t.Run("deleted_atがセットされemail/atnameが匿名値になる", func(t *testing.T) {
		// 行は (deleted_atで絞るFindByIDではなく) 直接クエリするため、論理削除・
		// 匿名化された値を観測できる。
		var deletedAt *time.Time
		var email, atname string
		if err := db.Writer.QueryRowContext(ctx,
			`SELECT deleted_at, email, atname FROM users WHERE id = ?`, int64(created.ID),
		).Scan(&deletedAt, &email, &atname); err != nil {
			t.Fatalf("行の取得に失敗: %v", err)
		}
		if deletedAt == nil {
			t.Error("deleted_atがセットされていない")
		}
		if email != anonEmail {
			t.Errorf("email = %q、期待値 = %q", email, anonEmail)
		}
		if atname != anonAtname {
			t.Errorf("atname = %q、期待値 = %q", atname, anonAtname)
		}
	})

	t.Run("論理削除後はFindByIDから外れる", func(t *testing.T) {
		user, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if user != nil {
			t.Error("論理削除後のFindByID() はnilを返すはず")
		}
	})

	t.Run("解放されたemailとatnameは別アカウントが再取得できる", func(t *testing.T) {
		if _, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "withdraw-me@example.com",
			Atname:   "withdrawme",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		}); err != nil {
			t.Errorf("解放されたemail/atnameでのCreate()のエラー = %v、期待値 = nil (再取得できるはず)", err)
		}
	})
}

// TestUserRepository_UpdateEmailはUpdateEmailがユーザーのemailを書き換えること、
// および別アカウントが既に使用しているアドレスへの変更がusers.emailのUNIQUE制約で失敗
// することを検証する (NOCASE照合により大文字小文字を区別しない)。
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
			t.Fatalf("Create()のエラー = %v", err)
		}

		if err := repo.UpdateEmail(ctx, created.ID, "after@example.com"); err != nil {
			t.Fatalf("UpdateEmail()のエラー = %v", err)
		}

		user, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("FindByID() = nil、期待値はユーザー")
		}
		if user.Email != "after@example.com" {
			t.Errorf("user.Email = %q、期待値 = %q", user.Email, "after@example.com")
		}
	})

	t.Run("既存アカウントと重複するアドレスへの更新はエラー", func(t *testing.T) {
		// 別アカウントがtaken@example.comを先に使用しているため、そのアドレスへの
		// 更新はusers.emailのUNIQUE制約 (NOCASE照合) で失敗する。
		if _, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "taken@example.com",
			Atname:   "takenemailuser",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		}); err != nil {
			t.Fatalf("既存ユーザーのCreate()のエラー = %v", err)
		}
		mover, err := repo.Create(ctx, repository.CreateUserInput{
			Email:    "mover@example.com",
			Atname:   "moveremailuser",
			Locale:   "ja",
			TimeZone: "Asia/Tokyo",
		})
		if err != nil {
			t.Fatalf("Create()のエラー = %v", err)
		}

		if err := repo.UpdateEmail(ctx, mover.ID, "Taken@Example.com"); err == nil {
			t.Error("重複アドレスへのUpdateEmail() はエラーになるはず")
		}
	})
}

// listAtnamesは、一覧に並んだユーザーのatnameをその順序のまま返す。テストが
// 期待するページを、その画面で読むことになるハンドルの並びとして書けるようにするため
// である。
func listAtnames(users []*model.User) []string {
	atnames := make([]string, len(users))
	for i, user := range users {
		atnames[i] = user.Atname
	}
	return atnames
}

// assertAtnamesは、一覧が指定したatnameを指定した順序でちょうど持たない限り
// テストを失敗させる。
func assertAtnames(t *testing.T, users []*model.User, want []string) {
	t.Helper()

	got := listAtnames(users)
	if len(got) != len(want) {
		t.Fatalf("一覧のatname = %v、期待値 = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("一覧のatname = %v、期待値 = %v", got, want)
		}
	}
}

func TestUserRepository_ListPageByAtnamePrefix(t *testing.T) {
	t.Parallel()

	t.Run("空のprefixは退会していない利用者を登録の新しい順に返す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("first").Build()
		testutil.NewUserBuilder(t, db).WithAtname("second").Build()
		testutil.NewUserBuilder(t, db).WithAtname("third").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
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
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}

		assertAtnames(t, users, []string{"staying"})
	})

	t.Run("limitとoffsetが1ページ分ずつを切り出す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("older").Build()
		testutil.NewUserBuilder(t, db).WithAtname("middle").Build()
		testutil.NewUserBuilder(t, db).WithAtname("newer").Build()

		first, err := repo.ListPageByAtnamePrefix(ctx, "", 2, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}
		assertAtnames(t, first, []string{"newer", "middle"})

		second, err := repo.ListPageByAtnamePrefix(ctx, "", 2, 2)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}
		assertAtnames(t, second, []string{"older"})
	})

	t.Run("最後のページを越えたoffsetは空を返す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("only").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "", 10, 50)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}

		assertAtnames(t, users, nil)
	})

	t.Run("prefixは前方一致で、大文字小文字を区別しない", func(t *testing.T) {
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
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}

		assertAtnames(t, users, []string{"alberta", "Alice"})
	})

	t.Run("prefixそのものと一致するatnameも含める", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("ada").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "ada", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}

		assertAtnames(t, users, []string{"ada"})
	})

	// アンダースコアはatnameの文字集合の一部であるため、それを含む検索はその文字を
	// 名指している。一致をLIKEで書けばこれはLIKEの1文字ワイルドカードになり、検索は
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
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}

		assertAtnames(t, users, []string{"a_b"})
	})

	t.Run("prefixに一致する利用者がいなければ空を返す", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("ada").Build()

		users, err := repo.ListPageByAtnamePrefix(ctx, "zoe", 10, 0)
		if err != nil {
			t.Fatalf("ListPageByAtnamePrefix()のエラー = %v", err)
		}

		assertAtnames(t, users, nil)
	})
}

func TestUserRepository_CountByAtnamePrefix(t *testing.T) {
	t.Parallel()

	t.Run("空のprefixは退会していない利用者を数える", func(t *testing.T) {
		t.Parallel()

		db := testutil.SetupDB(t)
		repo := repository.NewUserRepository(db)
		ctx := context.Background()
		testutil.NewUserBuilder(t, db).WithAtname("staying").Build()
		testutil.NewUserBuilder(t, db).WithAtname("alsostaying").Build()
		testutil.NewUserBuilder(t, db).WithAtname("leaving").WithDeletedAt(time.Now()).Build()

		count, err := repo.CountByAtnamePrefix(ctx, "")
		if err != nil {
			t.Fatalf("CountByAtnamePrefix()のエラー = %v", err)
		}
		if count != 2 {
			t.Errorf("CountByAtnamePrefix(\"\") = %d、期待値 = 2", count)
		}
	})

	t.Run("prefixで絞り込んだ件数を大文字小文字を区別せずに数える", func(t *testing.T) {
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
			t.Fatalf("CountByAtnamePrefix()のエラー = %v", err)
		}
		if count != 2 {
			t.Errorf("CountByAtnamePrefix(\"AL\") = %d、期待値 = 2", count)
		}
	})
}

// TestUserRepository_ListPageByAtnamePrefix_UsesTheAtnameIndexは、絞り込んだ一覧が
// atnameのUNIQUE制約を支える索引で答えられることを検証します。この検索は管理画面の
// ページを絞り込むものであり、答えるためにすべてのアカウントを読めば、その画面は
// コミュニティの人数が増えるほど遅くなります。
func TestUserRepository_ListPageByAtnamePrefix_UsesTheAtnameIndex(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := context.Background()

	statement := queryStatement(t, "users.sql", "ListUsersPageByAtnamePrefix")
	plan := queryPlan(t, ctx, db, statement, "a", "a\U0010FFFF", int64(50), int64(0))

	// SQLiteは、名前を与えられていないUNIQUE制約を支える索引を、その制約が立つ
	// テーブルの名前から名付ける。ここでusersの2つ目の制約にあたるのがatnameで
	// ある (1つ目はemail)。
	const index = "sqlite_autoindex_users_2"
	if !strings.Contains(plan, index) {
		t.Errorf("絞り込んだ一覧の実行計画 = %q、%q をたどるはず", plan, index)
	}
	if strings.Contains(plan, "SCAN users") {
		t.Errorf("絞り込んだ一覧の実行計画 = %q、usersの全走査を伴わないはず", plan)
	}
}
