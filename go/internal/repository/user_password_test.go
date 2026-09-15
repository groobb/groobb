package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserPasswordRepoはテストが所有するデータベース上にUserPasswordRepositoryを
// 作り、パスワードの所有ユーザーを作成してそのIDを返す。各テストが既存の所有者に
// パスワードを紐付けられるようにするためである。
func newUserPasswordRepo(t *testing.T) (*repository.UserPasswordRepository, model.UserID, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserPasswordRepository(db)
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	if userPassword.ID == 0 {
		t.Error("Create() userPassword.IDはDB採番で空でないはず")
	}
	if userPassword.UserID != userID {
		t.Errorf("userPassword.UserID = %v、期待値 = %v", userPassword.UserID, userID)
	}
	if userPassword.PasswordDigest != "$2a$04$digest.placeholder.value" {
		t.Errorf("userPassword.PasswordDigest = %q、期待値 = %q", userPassword.PasswordDigest, "$2a$04$digest.placeholder.value")
	}
	if userPassword.CreatedAt.IsZero() {
		t.Error("userPassword.CreatedAtはDB既定値で設定されるはず")
	}
	if userPassword.UpdatedAt.IsZero() {
		t.Error("userPassword.UpdatedAtはDB既定値で設定されるはず")
	}
}

func TestUserPasswordRepository_FindByUserID(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserPasswordRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$findable.digest.value",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("ユーザーIDでパスワードを取得できる", func(t *testing.T) {
		userPassword, err := repo.FindByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByUserID()のエラー = %v", err)
		}
		if userPassword == nil {
			t.Fatal("FindByUserID() = nil、期待値はパスワード")
		}
		if userPassword.UserID != userID {
			t.Errorf("userPassword.UserID = %v、期待値 = %v", userPassword.UserID, userID)
		}
		if userPassword.PasswordDigest != "$2a$04$findable.digest.value" {
			t.Errorf("userPassword.PasswordDigest = %q、期待値 = %q", userPassword.PasswordDigest, "$2a$04$findable.digest.value")
		}
	})

	t.Run("パスワードを持たないuser_idは (nil, nil) を返す", func(t *testing.T) {
		userPassword, err := repo.FindByUserID(ctx, model.UserID(testutil.UnusedID))
		if err != nil {
			t.Fatalf("FindByUserID()のエラー = %v、期待値 = nil", err)
		}
		if userPassword != nil {
			t.Errorf("FindByUserID() = %v、期待値 = nil", userPassword)
		}
	})
}

// TestUserPasswordRepository_UpdatePasswordDigestは、UpdatePasswordDigestが
// そのユーザーの保存ダイジェストを置き換え、後のFindByUserIDが新しい値を返すことを
// 検証する。
func TestUserPasswordRepository_UpdatePasswordDigest(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserPasswordRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$old.digest.value",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if err := repo.UpdatePasswordDigest(ctx, userID, "$2a$04$new.digest.value"); err != nil {
		t.Fatalf("UpdatePasswordDigest()のエラー = %v", err)
	}

	userPassword, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if userPassword == nil {
		t.Fatal("FindByUserID() = nil、期待値はパスワード")
	}
	if userPassword.PasswordDigest != "$2a$04$new.digest.value" {
		t.Errorf("userPassword.PasswordDigest = %q、期待値 = %q", userPassword.PasswordDigest, "$2a$04$new.digest.value")
	}
}

// TestUserPasswordRepository_CreateRejectsSecondPasswordは
// user_passwords.user_idのUNIQUE制約が、ユーザーあたり高々1つのパスワードを
// 強制することを確認する。
func TestUserPasswordRepository_CreateRejectsSecondPassword(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserPasswordRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$first.digest.value",
	}); err != nil {
		t.Fatalf("1回目のCreate()のエラー = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: "$2a$04$second.digest.value",
	})
	if err == nil {
		t.Error("同一ユーザーへの2つ目のパスワードCreate() はエラーになるはず")
	}
}
