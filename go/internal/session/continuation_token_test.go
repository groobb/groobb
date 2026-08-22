package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/testutil"
)

// TestManager_ExpiredContinuationToken verifies that the server-side expiry is
// enforced for both authentication flows even if a client keeps sending the
// Cookie after its browser lifetime.
//
// [Ja] TestManager_ExpiredContinuationToken は、クライアントがブラウザー上の有効期間後も
// Cookie を送信し続けた場合でも、両方の認証フローでサーバー側の期限が強制されることを
// 検証します。
func TestManager_ExpiredContinuationToken(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, testutil.NewTestConfig(t))
	expiredAt := time.Now().Add(-time.Minute)

	tests := []struct {
		name       string
		cookieName string
		token      string
		get        func(*http.Request) bool
	}{
		{
			name:       "email confirmation",
			cookieName: EmailConfirmationCookieName,
			token:      signContinuationToken(testutil.TestContinuationTokenKey, emailConfirmationTokenPurpose, 123, expiredAt),
			get: func(req *http.Request) bool {
				_, ok := mgr.GetEmailConfirmationID(req)
				return ok
			},
		},
		{
			name:       "two-factor pending",
			cookieName: TwoFactorPendingCookieName,
			token:      signContinuationToken(testutil.TestContinuationTokenKey, twoFactorPendingTokenPurpose, 456, expiredAt),
			get: func(req *http.Request) bool {
				_, ok := mgr.GetTwoFactorPendingUserID(req)
				return ok
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: tt.cookieName, Value: tt.token})
			if tt.get(req) {
				t.Error("expired continuation token was accepted")
			}
		})
	}
}

// TestContinuationToken_FailsClosedWithShortKey verifies that bypassing
// Config.Load with an invalid key cannot emit or accept continuation tokens.
//
// [Ja] TestContinuationToken_FailsClosedWithShortKey は、不正な鍵で Config.Load を迂回しても
// continuation token を発行・受理できないことを検証します。
func TestContinuationToken_FailsClosedWithShortKey(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(time.Minute)
	if token := signContinuationToken("short", emailConfirmationTokenPurpose, 123, expiresAt); token != "" {
		t.Errorf("signContinuationToken() = %q, want empty", token)
	}

	token := signContinuationToken(testutil.TestContinuationTokenKey, emailConfirmationTokenPurpose, 123, expiresAt)
	if _, ok := verifyContinuationToken("short", emailConfirmationTokenPurpose, token, time.Now()); ok {
		t.Error("verifyContinuationToken() ok = true with short key, want false")
	}
}
