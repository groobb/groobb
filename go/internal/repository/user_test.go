package repository_test

import (
	"context"
	"testing"

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
