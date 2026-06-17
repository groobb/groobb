package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestDeleteSessionUsecase_Execute_DeletesSession verifies that Execute deletes
// the session row, so the token no longer resolves to a session afterward.
//
// [Ja] TestDeleteSessionUsecase_Execute_DeletesSession は、Execute がセッション行を削除し、
// その後 token がセッションに解決しなくなることを検証します。
func TestDeleteSessionUsecase_Execute_DeletesSession(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userSessionRepo := repository.NewUserSessionRepository(query.New(db)).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()
	const token = "delete-session-token"
	if _, err := userSessionRepo.Create(context.Background(), repository.CreateUserSessionInput{
		UserID:    userID,
		Token:     token,
		IPAddress: "203.0.113.7",
		UserAgent: "test-agent",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	uc := usecase.NewDeleteSessionUsecase(userSessionRepo)
	if err := uc.Execute(context.Background(), token); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	session, err := userSessionRepo.FindByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("FindByToken() error = %v", err)
	}
	if session != nil {
		t.Error("削除後もセッションが残っている")
	}
}

// TestDeleteSessionUsecase_Execute_EmptyTokenIsNoop verifies that an empty token
// is a no-op (no error), so signing out when not signed in is harmless.
//
// [Ja] TestDeleteSessionUsecase_Execute_EmptyTokenIsNoop は、空の token が no-op
// (エラー無し) であり、未サインインでのサインアウトが無害であることを検証します。
func TestDeleteSessionUsecase_Execute_EmptyTokenIsNoop(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userSessionRepo := repository.NewUserSessionRepository(query.New(db)).WithTx(tx)

	uc := usecase.NewDeleteSessionUsecase(userSessionRepo)
	if err := uc.Execute(context.Background(), ""); err != nil {
		t.Errorf("Execute(\"\") error = %v, want nil", err)
	}
}
