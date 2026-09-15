package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestCreateSessionUsecase_Execute_Successは、Executeがトークンを生成して返し、
// そのユーザーのセッションを永続化し、tokenで同じユーザーに解決し直せることを検証します。
func TestCreateSessionUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userSessionRepo := repository.NewUserSessionRepository(db)
	uc := usecase.NewCreateSessionUsecase(userSessionRepo)

	userID := testutil.NewUserBuilder(t, db).Build()

	out, err := uc.Execute(context.Background(), usecase.CreateSessionInput{
		UserID:    userID,
		IPAddress: "203.0.113.7",
		UserAgent: "test-agent",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil || out.Token == "" {
		t.Fatal("Execute()のoutput / Token = 空")
	}

	// 永続化されたセッションはtokenで同じユーザーに解決し直せる。
	session, err := userSessionRepo.FindByToken(context.Background(), out.Token)
	if err != nil {
		t.Fatalf("FindByToken()のエラー = %v", err)
	}
	if session == nil {
		t.Fatal("生成したトークンでセッションを引けない (永続化されていない可能性)")
	}
	if session.UserID != userID {
		t.Errorf("session.UserID = %v、期待値 = %v", session.UserID, userID)
	}
}
