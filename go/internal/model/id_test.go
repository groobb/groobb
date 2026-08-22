package model_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestUserID_String verifies that a UserID stringifies to the decimal form of
// the int64 it wraps.
//
// [Ja] TestUserID_String は UserID がラップする int64 の 10 進表記で文字列化される
// ことを検証します。
func TestUserID_String(t *testing.T) {
	t.Parallel()

	id := model.UserID(42)

	if got, want := id.String(), "42"; got != want {
		t.Errorf("UserID.String() = %q, want %q", got, want)
	}
}

// TestUserTwoFactorAuthID_String verifies that a UserTwoFactorAuthID stringifies
// to the decimal form of the int64 it wraps.
//
// [Ja] TestUserTwoFactorAuthID_String は UserTwoFactorAuthID がラップする int64 の
// 10 進表記で文字列化されることを検証します。
func TestUserTwoFactorAuthID_String(t *testing.T) {
	t.Parallel()

	id := model.UserTwoFactorAuthID(7)

	if got, want := id.String(), "7"; got != want {
		t.Errorf("UserTwoFactorAuthID.String() = %q, want %q", got, want)
	}
}
