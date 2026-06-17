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

// newUserPasswordRepo builds a UserPasswordRepository bound to the test
// transaction (via WithTx) and creates a user to own the password, returning the
// user ID so writes roll back when the test finishes.
//
// [Ja] newUserPasswordRepo はテスト用トランザクションに束ねた (WithTx を通した)
// UserPasswordRepository を作り、パスワードの所有ユーザーを作成してその ID を返す。
// テスト終了時に書き込みはロールバックされる。
func newUserPasswordRepo(t *testing.T) (*repository.UserPasswordRepository, model.UserID, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	repo := repository.NewUserPasswordRepository(query.New(db)).WithTx(tx)
	return repo, userID, context.Background()
}

func TestUserPasswordRepository_Create(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserPasswordRepo(t)

	userPassword, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$digest.placeholder.value",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if userPassword.ID == (model.UserPasswordID{}) {
		t.Error("Create() userPassword.ID は DB 採番で空でないはず")
	}
	if userPassword.UserID != userID {
		t.Errorf("userPassword.UserID = %v, want %v", userPassword.UserID, userID)
	}
	if userPassword.PasswordDigest != "$2a$04$digest.placeholder.value" {
		t.Errorf("userPassword.PasswordDigest = %q, want %q", userPassword.PasswordDigest, "$2a$04$digest.placeholder.value")
	}
	if userPassword.CreatedAt.IsZero() {
		t.Error("userPassword.CreatedAt は DB 既定値で設定されるはず")
	}
	if userPassword.UpdatedAt.IsZero() {
		t.Error("userPassword.UpdatedAt は DB 既定値で設定されるはず")
	}
}

func TestUserPasswordRepository_FindByUserID(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserPasswordRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$findable.digest.value",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("ユーザー ID でパスワードを取得できる", func(t *testing.T) {
		userPassword, err := repo.FindByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByUserID() error = %v", err)
		}
		if userPassword == nil {
			t.Fatal("FindByUserID() = nil, want password")
		}
		if userPassword.UserID != userID {
			t.Errorf("userPassword.UserID = %v, want %v", userPassword.UserID, userID)
		}
		if userPassword.PasswordDigest != "$2a$04$findable.digest.value" {
			t.Errorf("userPassword.PasswordDigest = %q, want %q", userPassword.PasswordDigest, "$2a$04$findable.digest.value")
		}
	})

	t.Run("パスワードを持たない user_id は (nil, nil) を返す", func(t *testing.T) {
		userPassword, err := repo.FindByUserID(ctx, model.UserID(uuid.New()))
		if err != nil {
			t.Fatalf("FindByUserID() error = %v, want nil", err)
		}
		if userPassword != nil {
			t.Errorf("FindByUserID() = %v, want nil", userPassword)
		}
	})
}

// TestUserPasswordRepository_UpdatePasswordDigest verifies that
// UpdatePasswordDigest replaces the stored digest for the user, so a later
// FindByUserID returns the new value.
//
// [Ja] TestUserPasswordRepository_UpdatePasswordDigest は、UpdatePasswordDigest が
// そのユーザーの保存ダイジェストを置き換え、後の FindByUserID が新しい値を返すことを
// 検証する。
func TestUserPasswordRepository_UpdatePasswordDigest(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserPasswordRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$old.digest.value",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.UpdatePasswordDigest(ctx, userID, "$2a$04$new.digest.value"); err != nil {
		t.Fatalf("UpdatePasswordDigest() error = %v", err)
	}

	userPassword, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if userPassword == nil {
		t.Fatal("FindByUserID() = nil, want password")
	}
	if userPassword.PasswordDigest != "$2a$04$new.digest.value" {
		t.Errorf("userPassword.PasswordDigest = %q, want %q", userPassword.PasswordDigest, "$2a$04$new.digest.value")
	}
}

// TestUserPasswordRepository_CreateRejectsSecondPassword verifies the
// user_passwords.user_id UNIQUE constraint enforces at most one password per
// user.
//
// [Ja] TestUserPasswordRepository_CreateRejectsSecondPassword は
// user_passwords.user_id の UNIQUE 制約が、ユーザーあたり高々 1 つのパスワードを
// 強制することを確認する。
func TestUserPasswordRepository_CreateRejectsSecondPassword(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserPasswordRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$first.digest.value",
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$second.digest.value",
	})
	if err == nil {
		t.Error("同一ユーザーへの 2 つ目のパスワード Create() はエラーになるはず")
	}
}
