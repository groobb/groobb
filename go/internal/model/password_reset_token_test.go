package model_test

import (
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
)

// TestPasswordResetToken_IsUsedは、IsUsedがused_atが打刻されているとき
// (消費済みトークン) にちょうどtrueを、nilのとき (未使用トークン) にfalseを返すことを
// 検証します。
func TestPasswordResetToken_IsUsed(t *testing.T) {
	t.Parallel()

	usedAt := time.Now()
	tests := []struct {
		name   string
		usedAt *time.Time
		want   bool
	}{
		{name: "未使用 (used_atがnil)", usedAt: nil, want: false},
		{name: "使用済み (used_atが打刻済み)", usedAt: &usedAt, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token := &model.PasswordResetToken{UsedAt: tt.usedAt}
			if got := token.IsUsed(); got != tt.want {
				t.Errorf("IsUsed() = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}

// TestPasswordResetToken_IsExpiredは、IsExpiredがexpires_atが過去のときtrueを、
// 未来のときfalseを返すことを検証します。
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
				t.Errorf("IsExpired() = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}
