package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserSessionRepoはテストが所有するデータベース上にUserSessionRepositoryを
// 作り、セッションの所有ユーザーを作成してそのIDを返す。各テストが既存の所有者に
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	if userSession.ID == 0 {
		t.Error("Create() userSession.IDはDB採番で空でないはず")
	}
	if userSession.UserID != userID {
		t.Errorf("userSession.UserID = %v、期待値 = %v", userSession.UserID, userID)
	}
	if userSession.Token != "create-token" {
		t.Errorf("userSession.Token = %q、期待値 = %q", userSession.Token, "create-token")
	}
	if userSession.IPAddress != "203.0.113.1" {
		t.Errorf("userSession.IPAddress = %q、期待値 = %q", userSession.IPAddress, "203.0.113.1")
	}
	if userSession.UserAgent != "Mozilla/5.0 (Test)" {
		t.Errorf("userSession.UserAgent = %q、期待値 = %q", userSession.UserAgent, "Mozilla/5.0 (Test)")
	}
	if userSession.SignedInAt.IsZero() {
		t.Error("userSession.SignedInAtはDB既定値で設定されるはず")
	}
	if userSession.CreatedAt.IsZero() {
		t.Error("userSession.CreatedAtはDB既定値で設定されるはず")
	}
	if userSession.UpdatedAt.IsZero() {
		t.Error("userSession.UpdatedAtはDB既定値で設定されるはず")
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("トークンでセッションを取得できる", func(t *testing.T) {
		userSession, err := repo.FindByToken(ctx, "findable-token")
		if err != nil {
			t.Fatalf("FindByToken()のエラー = %v", err)
		}
		if userSession == nil {
			t.Fatal("FindByToken() = nil、期待値はセッション")
		}
		if userSession.UserID != userID {
			t.Errorf("userSession.UserID = %v、期待値 = %v", userSession.UserID, userID)
		}
		if userSession.Token != "findable-token" {
			t.Errorf("userSession.Token = %q、期待値 = %q", userSession.Token, "findable-token")
		}
	})

	t.Run("存在しないトークンは (nil, nil) を返す", func(t *testing.T) {
		userSession, err := repo.FindByToken(ctx, "missing-token")
		if err != nil {
			t.Fatalf("FindByToken()のエラー = %v、期待値 = nil", err)
		}
		if userSession != nil {
			t.Errorf("FindByToken() = %v、期待値 = nil", userSession)
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("トークンでセッションを削除できる", func(t *testing.T) {
		if err := repo.DeleteByToken(ctx, "deletable-token"); err != nil {
			t.Fatalf("DeleteByToken()のエラー = %v", err)
		}

		userSession, err := repo.FindByToken(ctx, "deletable-token")
		if err != nil {
			t.Fatalf("FindByToken()のエラー = %v", err)
		}
		if userSession != nil {
			t.Error("削除後のFindByToken() はnilを返すはず")
		}
	})

	t.Run("存在しないトークンの削除はエラーにならない", func(t *testing.T) {
		if err := repo.DeleteByToken(ctx, "never-existed-token"); err != nil {
			t.Errorf("存在しないトークンのDeleteByToken()のエラー = %v、期待値 = nil", err)
		}
	})
}

// TestUserSessionRepository_DeleteByUserIDはDeleteByUserIDが対象ユーザーの全
// セッションを削除し、他ユーザーのセッションには手を触れず、ユーザーがセッションを
// 持たないときは無害なno-opになることを確認する。
func TestUserSessionRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewUserSessionRepository(db)
	ctx := context.Background()

	// 2つのセッションを持つ対象ユーザーと、削除を生き延びるべきセッションを持つ
	// 別ユーザー。
	userID := testutil.NewUserBuilder(t, db).Build()
	for _, token := range []string{"del-by-user-1", "del-by-user-2"} {
		if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
			UserID: userID, Token: token, IPAddress: "203.0.113.6", UserAgent: "agent",
		}); err != nil {
			t.Fatalf("Create()のエラー = %v", err)
		}
	}
	otherUserID := testutil.NewUserBuilder(t, db).Build()
	if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID: otherUserID, Token: "other-user-token", IPAddress: "203.0.113.7", UserAgent: "agent",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if err := repo.DeleteByUserID(ctx, userID); err != nil {
		t.Fatalf("DeleteByUserID()のエラー = %v", err)
	}

	t.Run("対象ユーザーの全セッションが削除される", func(t *testing.T) {
		for _, token := range []string{"del-by-user-1", "del-by-user-2"} {
			session, err := repo.FindByToken(ctx, token)
			if err != nil {
				t.Fatalf("FindByToken()のエラー = %v", err)
			}
			if session != nil {
				t.Errorf("token %q は削除後に残っている", token)
			}
		}
	})

	t.Run("他ユーザーのセッションは残る", func(t *testing.T) {
		session, err := repo.FindByToken(ctx, "other-user-token")
		if err != nil {
			t.Fatalf("FindByToken()のエラー = %v", err)
		}
		if session == nil {
			t.Error("他ユーザーのセッションが削除された")
		}
	})

	t.Run("セッションを持たないユーザーの削除はエラーにならない", func(t *testing.T) {
		if err := repo.DeleteByUserID(ctx, userID); err != nil {
			t.Errorf("セッションが無いユーザーのDeleteByUserID()のエラー = %v、期待値 = nil", err)
		}
	})
}

// TestUserSessionRepository_CreateRejectsDuplicateTokenは
// user_sessions.tokenのUNIQUE制約がエラーとして表面化することを確認する。
func TestUserSessionRepository_CreateRejectsDuplicateToken(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserSessionRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     "dup-token",
		IPAddress: "203.0.113.4",
		UserAgent: "agent",
	}); err != nil {
		t.Fatalf("1回目のCreate()のエラー = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     "dup-token",
		IPAddress: "203.0.113.5",
		UserAgent: "agent",
	})
	if err == nil {
		t.Error("重複トークンのCreate() はエラーになるはず")
	}
}
