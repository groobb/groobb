package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserSessionRepo builds a UserSessionRepository bound to the test
// transaction (via WithTx) and creates a user to own the sessions, returning the
// user ID so writes roll back when the test finishes.
//
// [Ja] newUserSessionRepo はテスト用トランザクションに束ねた (WithTx を通した)
// UserSessionRepository を作り、セッションの所有ユーザーを作成してその ID を返す。
// テスト終了時に書き込みはロールバックされる。
func newUserSessionRepo(t *testing.T) (*repository.UserSessionRepository, model.UserID, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	repo := repository.NewUserSessionRepository(query.New(db)).WithTx(tx)
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

	if userSession.ID == (model.UserSessionID{}) {
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
