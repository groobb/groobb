package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/testutil"
)

// TestManager_ExpiredContinuationTokenは、クライアントがブラウザー上の有効期間後も
// Cookieを送信し続けた場合でも、両方の認証フローでサーバー側の期限が強制されることを
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
			name:       "メール確認",
			cookieName: EmailConfirmationCookieName,
			token:      signContinuationToken(testutil.TestContinuationTokenKey, emailConfirmationTokenPurpose, 123, expiredAt),
			get: func(req *http.Request) bool {
				_, ok := mgr.GetEmailConfirmationID(req)
				return ok
			},
		},
		{
			name:       "2段階認証の保留",
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
				t.Error("期限切れのcontinuation tokenが受理された")
			}
		})
	}
}

// TestContinuationToken_FailsClosedWithShortKeyは、不正な鍵でConfig.Loadを迂回しても
// continuation tokenを発行・受理できないことを検証します。
func TestContinuationToken_FailsClosedWithShortKey(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(time.Minute)
	if token := signContinuationToken("short", emailConfirmationTokenPurpose, 123, expiresAt); token != "" {
		t.Errorf("signContinuationToken() = %q、期待値は空文字列", token)
	}

	token := signContinuationToken(testutil.TestContinuationTokenKey, emailConfirmationTokenPurpose, 123, expiresAt)
	if _, ok := verifyContinuationToken("short", emailConfirmationTokenPurpose, token, time.Now()); ok {
		t.Error("短い鍵でのverifyContinuationToken()のok = true、期待値 = false")
	}
}
