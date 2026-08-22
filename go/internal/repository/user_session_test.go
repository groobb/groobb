package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserSessionRepo builds a UserSessionRepository over the database the test
// owns and creates a user to own the sessions, returning the user ID so each
// test can attach its sessions to an existing owner.
//
// [Ja] newUserSessionRepo はテストが所有するデータベース上に UserSessionRepository を
// 作り、セッションの所有ユーザーを作成してその ID を返す。各テストが既存の所有者に
// セッションを紐付けられるようにするためである。
func newUserSessionRepo(t *testing.T) (*repository.UserSessionRepository, model.UserID, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserSessionRepository(db)
	return repo, userID, context.Background()
}

func TestUserSessionRepository_Create(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserSessionRepo(t)

	userSession, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     "create-token",
		IPAddress: "203.0.113.1",
		UserAgent: "Mozilla/5.0 (Test)",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if userSession.ID == 0 {
		t.Error("Create() userSession.ID は DB 採番で空でないはず")
	}
	if userSession.UserID != userID {
		t.Errorf("userSession.UserID = %v, want %v", userSession.UserID, userID)
	}
	if userSession.Token != "create-token" {
		t.Errorf("userSession.Token = %q, want %q", userSession.Token, "create-token")
	}
	if userSession.IPAddress != "203.0.113.1" {
		t.Errorf("userSession.IPAddress = %q, want %q", userSession.IPAddress, "203.0.113.1")
	}
	if userSession.UserAgent != "Mozilla/5.0 (Test)" {
		t.Errorf("userSession.UserAgent = %q, want %q", userSession.UserAgent, "Mozilla/5.0 (Test)")
	}
	if userSession.SignedInAt.IsZero() {
		t.Error("userSession.SignedInAt は DB 既定値で設定されるはず")
	}
	if userSession.CreatedAt.IsZero() {
		t.Error("userSession.CreatedAt は DB 既定値で設定されるはず")
	}
	if userSession.UpdatedAt.IsZero() {
		t.Error("userSession.UpdatedAt は DB 既定値で設定されるはず")
	}
}

func TestUserSessionRepository_FindByToken(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserSessionRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     "findable-token",
		IPAddress: "203.0.113.2",
		UserAgent: "agent",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("トークンでセッションを取得できる", func(t *testing.T) {
		userSession, err := repo.FindByToken(ctx, "findable-token")
		if err != nil {
			t.Fatalf("FindByToken() error = %v", err)
		}
		if userSession == nil {
			t.Fatal("FindByToken() = nil, want session")
		}
		if userSession.UserID != userID {
			t.Errorf("userSession.UserID = %v, want %v", userSession.UserID, userID)
		}
		if userSession.Token != "findable-token" {
			t.Errorf("userSession.Token = %q, want %q", userSession.Token, "findable-token")
		}
	})

	t.Run("存在しないトークンは (nil, nil) を返す", func(t *testing.T) {
		userSession, err := repo.FindByToken(ctx, "missing-token")
		if err != nil {
			t.Fatalf("FindByToken() error = %v, want nil", err)
		}
		if userSession != nil {
			t.Errorf("FindByToken() = %v, want nil", userSession)
		}
	})
}

func TestUserSessionRepository_DeleteByToken(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserSessionRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     "deletable-token",
		IPAddress: "203.0.113.3",
		UserAgent: "agent",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("トークンでセッションを削除できる", func(t *testing.T) {
		if err := repo.DeleteByToken(ctx, "deletable-token"); err != nil {
			t.Fatalf("DeleteByToken() error = %v", err)
		}

		userSession, err := repo.FindByToken(ctx, "deletable-token")
		if err != nil {
			t.Fatalf("FindByToken() error = %v", err)
		}
		if userSession != nil {
			t.Error("削除後の FindByToken() は nil を返すはず")
		}
	})

	t.Run("存在しないトークンの削除はエラーにならない", func(t *testing.T) {
		if err := repo.DeleteByToken(ctx, "never-existed-token"); err != nil {
			t.Errorf("存在しないトークンの DeleteByToken() error = %v, want nil", err)
		}
	})
}

// TestUserSessionRepository_DeleteByUserID verifies that DeleteByUserID removes
// every session owned by the target user, leaves other users' sessions untouched,
// and is a harmless no-op when the user has no sessions.
//
// [Ja] TestUserSessionRepository_DeleteByUserID は DeleteByUserID が対象ユーザーの全
// セッションを削除し、他ユーザーのセッションには手を触れず、ユーザーがセッションを
// 持たないときは無害な no-op になることを確認する。
func TestUserSessionRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserSessionRepository(db)
	ctx := context.Background()

	// The target user with two sessions, plus another user whose session must
	// survive the delete.
	//
	// [Ja] 2 つのセッションを持つ対象ユーザーと、削除を生き延びるべきセッションを持つ
	// 別ユーザー。
	userID := testutil.NewUserBuilder(t, db).Build()
	for _, token := range []string{"del-by-user-1", "del-by-user-2"} {
		if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
			UserID: userID, Token: token, IPAddress: "203.0.113.6", UserAgent: "agent",
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	otherUserID := testutil.NewUserBuilder(t, db).Build()
	if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID: otherUserID, Token: "other-user-token", IPAddress: "203.0.113.7", UserAgent: "agent",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.DeleteByUserID(ctx, userID); err != nil {
		t.Fatalf("DeleteByUserID() error = %v", err)
	}

	t.Run("対象ユーザーの全セッションが削除される", func(t *testing.T) {
		for _, token := range []string{"del-by-user-1", "del-by-user-2"} {
			session, err := repo.FindByToken(ctx, token)
			if err != nil {
				t.Fatalf("FindByToken() error = %v", err)
			}
			if session != nil {
				t.Errorf("token %q は削除後に残っている", token)
			}
		}
	})

	t.Run("他ユーザーのセッションは残る", func(t *testing.T) {
		session, err := repo.FindByToken(ctx, "other-user-token")
		if err != nil {
			t.Fatalf("FindByToken() error = %v", err)
		}
		if session == nil {
			t.Error("他ユーザーのセッションが削除された")
		}
	})

	t.Run("セッションを持たないユーザーの削除はエラーにならない", func(t *testing.T) {
		if err := repo.DeleteByUserID(ctx, userID); err != nil {
			t.Errorf("セッションが無いユーザーの DeleteByUserID() error = %v, want nil", err)
		}
	})
}

// TestUserSessionRepository_CreateRejectsDuplicateToken verifies the
// user_sessions.token UNIQUE constraint surfaces as an error.
//
// [Ja] TestUserSessionRepository_CreateRejectsDuplicateToken は
// user_sessions.token の UNIQUE 制約がエラーとして表面化することを確認する。
func TestUserSessionRepository_CreateRejectsDuplicateToken(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserSessionRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     "dup-token",
		IPAddress: "203.0.113.4",
		UserAgent: "agent",
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     "dup-token",
		IPAddress: "203.0.113.5",
		UserAgent: "agent",
	})
	if err == nil {
		t.Error("重複トークンの Create() はエラーになるはず")
	}
}
