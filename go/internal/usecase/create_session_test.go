package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestCreateSessionUsecase_Execute_Success verifies that Execute generates a
// token, returns it, and persists a session for the user that resolves back to
// that user by token.
//
// [Ja] TestCreateSessionUsecase_Execute_Success は、Execute がトークンを生成して返し、
// そのユーザーのセッションを永続化し、token で同じユーザーに解決し直せることを検証します。
func TestCreateSessionUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userSessionRepo := repository.NewUserSessionRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewCreateSessionUsecase(userSessionRepo)

	userID := testutil.NewUserBuilder(t, tx).Build()

	out, err := uc.Execute(context.Background(), usecase.CreateSessionInput{
		UserID:    userID,
		IPAddress: "203.0.113.7",
		UserAgent: "test-agent",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.Token == "" {
		t.Fatal("Execute() output / Token = empty")
	}

	// The persisted session resolves back to the same user by its token.
	//
	// [Ja] 永続化されたセッションは token で同じユーザーに解決し直せる。
	session, err := userSessionRepo.FindByToken(context.Background(), out.Token)
	if err != nil {
		t.Fatalf("FindByToken() error = %v", err)
	}
	if session == nil {
		t.Fatal("生成したトークンでセッションを引けない (永続化されていない可能性)")
	}
	if session.UserID != userID {
		t.Errorf("session.UserID = %v, want %v", session.UserID, userID)
	}
}
