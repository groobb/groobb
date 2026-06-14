package model_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestValidationError covers accumulating field and global messages and the
// accessors a handler / template uses to render them.
//
// [Ja] TestValidationError はフィールド・グローバルメッセージの蓄積と、ハンドラーや
// テンプレートが描画に使うアクセサを検証します。
func TestValidationError(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()
	if ve.HasErrors() {
		t.Fatal("a freshly created ValidationError should have no errors")
	}

	ve.AddField("email", "is required")
	ve.AddGlobal("the form has errors")

	if !ve.HasErrors() {
		t.Error("HasErrors() = false, want true after adding errors")
	}
	if !ve.HasFieldError("email") {
		t.Error("HasFieldError(email) = false, want true")
	}
	if got := ve.GetFieldErrors("email"); len(got) != 1 || got[0] != "is required" {
		t.Errorf("GetFieldErrors(email) = %v, want [is required]", got)
	}
	if got := ve.FieldErrors(); len(got) != 1 || got[0].Field != "email" {
		t.Errorf("FieldErrors() = %v, want a single entry for email", got)
	}
	if ve.Error() != "validation failed" {
		t.Errorf("Error() = %q, want %q", ve.Error(), "validation failed")
	}
}

// TestAsValidationError verifies that a wrapped *ValidationError is recovered
// and that an unrelated error is not mistaken for one.
//
// [Ja] TestAsValidationError はラップされた *ValidationError が取り出せること、
// および無関係なエラーがそれと誤認されないことを検証します。
func TestAsValidationError(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()
	ve.AddGlobal("x")
	wrapped := fmt.Errorf("wrapped: %w", ve)

	if got := model.AsValidationError(wrapped); got == nil {
		t.Error("AsValidationError() should recover a wrapped *ValidationError")
	}
	if got := model.AsValidationError(errors.New("plain")); got != nil {
		t.Error("AsValidationError() should return nil for a non-ValidationError")
	}
}

// TestAppError verifies the SafeError behaviour: Error() exposes only the
// user-safe message, while the internal cause stays reachable through Unwrap and
// LogString.
//
// [Ja] TestAppError は SafeError の挙動を検証します。Error() はユーザー安全な
// メッセージのみを公開し、内部原因は Unwrap と LogString からは参照できます。
func TestAppError(t *testing.T) {
	t.Parallel()

	internal := errors.New("connection reset by peer")
	ae := &model.AppError{
		Code:     model.AppErrCodeResourceNotFound,
		UserMsg:  "not found",
		Internal: internal,
		Metadata: map[string]string{"user_id": "u1"},
	}

	if ae.Error() != "not found" {
		t.Errorf("Error() = %q, want only the user-safe message", ae.Error())
	}
	if strings.Contains(ae.Error(), "connection reset") {
		t.Error("Error() leaked the internal cause to the user-facing message")
	}
	if !errors.Is(ae, internal) {
		t.Error("Unwrap() should expose the internal error to errors.Is")
	}
	if !strings.Contains(ae.LogString(), "connection reset") {
		t.Errorf("LogString() = %q, want it to include the internal cause", ae.LogString())
	}
	if got := model.AsAppError(fmt.Errorf("wrapped: %w", ae)); got == nil {
		t.Error("AsAppError() should recover a wrapped *AppError")
	}
}
