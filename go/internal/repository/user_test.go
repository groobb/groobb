package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserRepo builds a UserRepository bound to the test transaction, exercising
// WithTx so writes roll back when the test finishes.
//
// [Ja] newUserRepo はテスト用トランザクションに束ねた UserRepository を作る。
// WithTx を通すことで、テスト終了時に書き込みがロールバックされる。
func newUserRepo(t *testing.T) (*repository.UserRepository, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	return repository.NewUserRepository(query.New(db)).WithTx(tx), context.Background()
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

	if user.ID == (model.UserID{}) {
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
		user, err := repo.FindByID(ctx, model.UserID(uuid.New()))
		if err != nil {
			t.Fatalf("FindByID() error = %v, want nil", err)
		}
		if user != nil {
			t.Errorf("FindByID() = %v, want nil", user)
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

	t.Run("citext により大文字小文字を無視して取得できる", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, "FindByEmail@Example.com")
		if err != nil {
			t.Fatalf("FindByEmail() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByEmail() = nil, want user (citext は大文字小文字を無視するはず)")
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

func TestUserRepository_FindBySessionToken(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, tx).
		WithEmail("session-user@example.com").
		Build()
	testutil.NewUserSessionBuilder(t, tx).
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
// constraint surfaces as an error (case-insensitive via citext).
//
// [Ja] TestUserRepository_CreateRejectsDuplicateEmail は users.email の UNIQUE 制約が
// エラーとして表面化することを確認する (citext により大文字小文字を区別しない)。
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
// resolves the user, the match is case-insensitive via citext, and an unknown
// atname returns (nil, nil).
//
// [Ja] TestUserRepository_FindByAtname は atname による取得を検証する。存在する atname は
// ユーザーを解決し、照合は citext により大文字小文字を無視し、未知の atname は (nil, nil)
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

	t.Run("citext により大文字小文字を無視して取得できる", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, "FindByAtnameUser")
		if err != nil {
			t.Fatalf("FindByAtname() error = %v", err)
		}
		if user == nil {
			t.Fatal("FindByAtname() = nil, want user (citext は大文字小文字を無視するはず)")
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
// constraint surfaces as an error (case-insensitive via citext).
//
// [Ja] TestUserRepository_CreateRejectsDuplicateAtname は users.atname の UNIQUE 制約が
// エラーとして表面化することを確認する (citext により大文字小文字を区別しない)。
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

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	// A time in the past stands in for the moment the user withdrew; the exact
	// value does not matter, only that deleted_at is non-null.
	//
	// [Ja] 過去の時刻をユーザーが退会した時点の代わりに使う。deleted_at が非 NULL である
	// ことだけが重要で、具体的な値は問わない。
	deletedAt := time.Now().Add(-24 * time.Hour)

	t.Run("FindByID は論理削除済みユーザーを除外する", func(t *testing.T) {
		id := testutil.NewUserBuilder(t, tx).WithDeletedAt(deletedAt).Build()

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
		testutil.NewUserBuilder(t, tx).WithEmail(email).WithDeletedAt(deletedAt).Build()

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
		testutil.NewUserBuilder(t, tx).WithAtname(atname).WithDeletedAt(deletedAt).Build()

		user, err := repo.FindByAtname(ctx, atname)
		if err != nil {
			t.Fatalf("FindByAtname() error = %v", err)
		}
		if user != nil {
			t.Errorf("FindByAtname() = %v, want nil (論理削除済みは除外されるはず)", user)
		}
	})

	t.Run("FindBySessionToken は論理削除済みユーザーのセッションを解決しない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).WithDeletedAt(deletedAt).Build()
		testutil.NewUserSessionBuilder(t, tx).
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

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserRepository(query.New(db)).WithTx(tx)
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
	anonAtname := "deleted_" + strings.ReplaceAll(created.ID.String(), "-", "")

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
		if err := tx.QueryRow(ctx,
			`SELECT deleted_at, email, atname FROM users WHERE id = $1`, uuid.UUID(created.ID),
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
// users.email UNIQUE constraint (case-insensitive via citext).
//
// [Ja] TestUserRepository_UpdateEmail は UpdateEmail がユーザーの email を書き換えること、
// および別アカウントが既に使用しているアドレスへの変更が users.email の UNIQUE 制約で失敗
// することを検証する (citext により大文字小文字を区別しない)。
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
		// on the users.email UNIQUE constraint (citext, case-insensitive).
		//
		// [Ja] 別アカウントが taken@example.com を先に使用しているため、そのアドレスへの
		// 更新は users.email の UNIQUE 制約 (citext) で失敗する。
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
