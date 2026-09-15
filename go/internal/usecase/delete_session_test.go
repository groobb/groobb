package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestDeleteSessionUsecase_Execute_DeletesSessionは、Executeがセッション行を削除し、
// その後tokenがセッションに解決しなくなることを検証します。
func TestDeleteSessionUsecase_Execute_DeletesSession(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userSessionRepo := repository.NewUserSessionRepository(db)

	userID := testutil.NewUserBuilder(t, db).Build()
	const token = "delete-session-token"
	if _, err := userSessionRepo.Create(context.Background(), repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     token,
		IPAddress: "203.0.113.7",
		UserAgent: "test-agent",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	uc := usecase.NewDeleteSessionUsecase(userSessionRepo)
	if err := uc.Execute(context.Background(), token); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	session, err := userSessionRepo.FindByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("FindByToken()のエラー = %v", err)
	}
	if session != nil {
		t.Error("削除後もセッションが残っている")
	}
}

// TestDeleteSessionUsecase_Execute_EmptyTokenIsNoopは、空のtokenがno-op
// (エラー無し) であり、未サインインでのサインアウトが無害であることを検証します。
func TestDeleteSessionUsecase_Execute_EmptyTokenIsNoop(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userSessionRepo := repository.NewUserSessionRepository(db)

	uc := usecase.NewDeleteSessionUsecase(userSessionRepo)
	if err := uc.Execute(context.Background(), ""); err != nil {
		t.Errorf("Execute(\"\")のエラー = %v、期待値 = nil", err)
	}
}
