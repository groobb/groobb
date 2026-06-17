package model_test

import (
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
)

// TestPasswordResetToken_IsUsed verifies that IsUsed reports true exactly when
// used_at is stamped (a spent token) and false when it is nil (an unused token).
//
// [Ja] TestPasswordResetToken_IsUsed は、IsUsed が used_at が打刻されているとき
// (消費済みトークン) にちょうど true を、nil のとき (未使用トークン) に false を返すことを
// 検証します。
func TestPasswordResetToken_IsUsed(t *testing.T) {
	t.Parallel()

	usedAt := time.Now()
	tests := []struct {
		name   string
		usedAt *time.Time
		want   bool
	}{
		{name: "未使用 (used_at が nil)", usedAt: nil, want: false},
		{name: "使用済み (used_at が打刻済み)", usedAt: &usedAt, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token := &model.PasswordResetToken{UsedAt: tt.usedAt}
			if got := token.IsUsed(); got != tt.want {
				t.Errorf("IsUsed() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPasswordResetToken_IsExpired verifies that IsExpired reports true when
// expires_at is in the past and false when it is in the future.
//
// [Ja] TestPasswordResetToken_IsExpired は、IsExpired が expires_at が過去のとき true を、
// 未来のとき false を返すことを検証します。
func TestPasswordResetToken_IsExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{name: "未来の有効期限はまだ有効", expiresAt: time.Now().Add(time.Hour), want: false},
		{name: "過去の有効期限は期限切れ", expiresAt: time.Now().Add(-time.Hour), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token := &model.PasswordResetToken{ExpiresAt: tt.expiresAt}
			if got := token.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}
