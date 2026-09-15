package model_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestValidationErrorはフィールド・グローバルメッセージの蓄積と、ハンドラーや
// テンプレートが描画に使うアクセサを検証します。
func TestValidationError(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()
	if ve.HasErrors() {
		t.Fatal("作成直後のValidationErrorがエラーを持っている")
	}
	if ve.HasFieldErrors() {
		t.Error("フィールドエラーを追加する前のHasFieldErrors() = true、期待値 = false")
	}

	ve.AddField("email", "is required")
	ve.AddGlobal("the form has errors")

	if !ve.HasErrors() {
		t.Error("エラーを追加した後のHasErrors() = false、期待値 = true")
	}
	if !ve.HasFieldErrors() {
		t.Error("フィールドエラーを追加した後のHasFieldErrors() = false、期待値 = true")
	}
	if !ve.HasFieldError("email") {
		t.Error("HasFieldError(email) = false、期待値 = true")
	}
	if got := ve.GetFieldErrors("email"); len(got) != 1 || got[0] != "is required" {
		t.Errorf("GetFieldErrors(email) = %v、期待値 = [is required]", got)
	}
	if got := ve.FieldErrors(); len(got) != 1 || got[0].Field != "email" {
		t.Errorf("FieldErrors() = %v、期待値はemailの1件だけ", got)
	}
	if ve.Error() != "validation failed" {
		t.Errorf("Error() = %q、期待値 = %q", ve.Error(), "validation failed")
	}
}

// TestValidationError_NilReceiverは、各アクセサがnilの *ValidationErrorに対して
// 空のものと同じ答えを返すことを検証します。
//
// ページは送信が拒否されるまでエラーを持たないため、テンプレートは初回描画のたびにこれらを
// nilに対して呼びます。呼び出し側がたまたま尋ねる順序ではなく、それ自体で成り立つ必要が
// あります。
func TestValidationError_NilReceiver(t *testing.T) {
	t.Parallel()

	var ve *model.ValidationError

	if ve.HasErrors() {
		t.Error("nilのValidationErrorに対するHasErrors() = true、期待値 = false")
	}
	if ve.HasGlobalError() {
		t.Error("nilのValidationErrorに対するHasGlobalError() = true、期待値 = false")
	}
	if ve.HasFieldErrors() {
		t.Error("nilのValidationErrorに対するHasFieldErrors() = true、期待値 = false")
	}
	if ve.HasFieldError("email") {
		t.Error("nilのValidationErrorに対するHasFieldError(email) = true、期待値 = false")
	}
	if got := ve.GetGlobalErrors(); got != nil {
		t.Errorf("nilのValidationErrorに対するGetGlobalErrors() = %v、期待値 = nil", got)
	}
	if got := ve.GetFieldErrors("email"); got != nil {
		t.Errorf("nilのValidationErrorに対するGetFieldErrors(email) = %v、期待値 = nil", got)
	}
	if got := ve.FieldErrors(); got != nil {
		t.Errorf("nilのValidationErrorに対するFieldErrors() = %v、期待値 = nil", got)
	}
}

// TestAsValidationErrorはラップされた *ValidationErrorが取り出せること、
// および無関係なエラーがそれと誤認されないことを検証します。
func TestAsValidationError(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()
	ve.AddGlobal("x")
	wrapped := fmt.Errorf("wrapped: %w", ve)

	if got := model.AsValidationError(wrapped); got == nil {
		t.Error("AsValidationError()がラップされた *ValidationErrorを取り出せなかった")
	}
	if got := model.AsValidationError(errors.New("plain")); got != nil {
		t.Error("ValidationErrorでないエラーに対するAsValidationError()がnilを返さなかった")
	}
}

// TestAppErrorはSafeErrorの挙動を検証します。Error() はユーザー安全な
// メッセージのみを公開し、内部原因はUnwrapとLogStringからは参照できます。
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
		t.Errorf("Error() = %q、期待値はユーザー安全なメッセージのみ", ae.Error())
	}
	if strings.Contains(ae.Error(), "connection reset") {
		t.Error("Error()がユーザー向けメッセージに内部原因を漏らした")
	}
	if !errors.Is(ae, internal) {
		t.Error("Unwrap()が内部エラーをerrors.Isに公開していない")
	}
	if !strings.Contains(ae.LogString(), "connection reset") {
		t.Errorf("LogString() = %q、内部原因を含むことを期待", ae.LogString())
	}
	if got := model.AsAppError(fmt.Errorf("wrapped: %w", ae)); got == nil {
		t.Error("AsAppError()がラップされた *AppErrorを取り出せなかった")
	}
}
