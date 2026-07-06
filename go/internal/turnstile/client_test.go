package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockTransport redirects every request to target (an httptest server) so the
// Client can be tested without changing the hardcoded siteverifyURL constant.
//
// [Ja] mockTransport はすべてのリクエストを target (httptest サーバー) に振り向け、
// ハードコードされた siteverifyURL 定数を変えずに Client をテストできるようにする。
type mockTransport struct {
	target string
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.target, "http://")
	req.URL.Path = ""
	return http.DefaultTransport.RoundTrip(req)
}

// blockedTransport fails the test if any request is attempted. The bypass paths
// (empty secret, empty token) must return without contacting siteverify.
//
// [Ja] blockedTransport はリクエストが試みられたらテストを失敗させる。バイパス経路
// (空シークレット・空トークン) は siteverify に接続せず返す必要がある。
type blockedTransport struct {
	t *testing.T
}

func (bt *blockedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	bt.t.Errorf("Verify contacted siteverify on a bypass path, want no HTTP request")
	return nil, fmt.Errorf("unexpected request")
}

// TestVerify_Success verifies that a success response yields (true, nil).
//
// [Ja] TestVerify_Success は成功レスポンスで (true, nil) が返ることを検証する。
func TestVerify_Success(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		// Assert the request carries the secret and token under the JSON field
		// names siteverify expects, so a drift in the verifyRequest tags (which
		// only the live API would reject) is caught here.
		//
		// [Ja] リクエストが secret とトークンを siteverify の要求する JSON フィールド名で
		// 運んでいることを検証する。verifyRequest のタグがずれると本番 API でしか弾かれない
		// ため、ここで捕捉する。
		var reqBody verifyRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if reqBody.Secret != "test-secret-key" {
			t.Errorf("request secret = %q, want %q", reqBody.Secret, "test-secret-key")
		}
		if reqBody.Response != "test-token" {
			t.Errorf("request response = %q, want %q", reqBody.Response, "test-token")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "challenge_ts": "2026-01-01T00:00:00Z", "hostname": "groobb.example.dev"}`))
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}

	success, err := client.Verify(context.Background(), "test-token")
	if err != nil {
		t.Errorf("Verify() error = %v, want nil", err)
	}
	if !success {
		t.Errorf("Verify() success = %v, want true", success)
	}
}

// TestVerify_Failure verifies that a success=false response yields an error.
//
// [Ja] TestVerify_Failure は success=false のレスポンスで error が返ることを検証する。
func TestVerify_Failure(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": false}`))
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}

	success, err := client.Verify(context.Background(), "invalid-token")
	if err == nil {
		t.Error("Verify() error = nil, want error")
	}
	if success {
		t.Errorf("Verify() success = %v, want false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "turnstile 検証に失敗しました") {
		t.Errorf("Verify() error = %v, want it to contain %q", err, "turnstile 検証に失敗しました")
	}
}

// TestVerify_FailureWithErrorCodes verifies that error-codes are surfaced in the
// returned error.
//
// [Ja] TestVerify_FailureWithErrorCodes は error-codes が返り値の error に含まれる
// ことを検証する。
func TestVerify_FailureWithErrorCodes(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": false, "error-codes": ["invalid-input-response", "timeout-or-duplicate"]}`))
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}

	success, err := client.Verify(context.Background(), "invalid-token")
	if err == nil {
		t.Error("Verify() error = nil, want error")
	}
	if success {
		t.Errorf("Verify() success = %v, want false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "エラーコード") {
		t.Errorf("Verify() error = %v, want it to contain %q", err, "エラーコード")
	}
}

// TestVerify_EmptyToken verifies that an empty token is an expected non-pass:
// (false, nil) is returned without contacting siteverify.
//
// [Ja] TestVerify_EmptyToken は空トークンが想定内の非通過であることを検証する。
// siteverify に接続せず (false, nil) を返す。
func TestVerify_EmptyToken(t *testing.T) {
	t.Parallel()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &blockedTransport{t: t}

	success, err := client.Verify(context.Background(), "")
	if err != nil {
		t.Errorf("Verify() error = %v, want nil", err)
	}
	if success {
		t.Errorf("Verify() success = %v, want false", success)
	}
}

// TestVerify_EmptySecretBypass verifies that an empty secret key bypasses
// verification: (true, nil) is returned without contacting siteverify.
//
// [Ja] TestVerify_EmptySecretBypass は空のシークレットキーが検証をバイパスする
// ことを検証する。siteverify に接続せず (true, nil) を返す。
func TestVerify_EmptySecretBypass(t *testing.T) {
	t.Parallel()

	client := NewClient("")
	client.httpClient.Transport = &blockedTransport{t: t}

	success, err := client.Verify(context.Background(), "any-token")
	if err != nil {
		t.Errorf("Verify() error = %v, want nil", err)
	}
	if !success {
		t.Errorf("Verify() success = %v, want true", success)
	}
}

// TestVerify_Timeout verifies that Verify fails with an error when siteverify
// does not respond within the client's timeout.
//
// [Ja] TestVerify_Timeout は、siteverify がクライアントのタイムアウト内に応答しない
// とき Verify が error で失敗することを検証する。
func TestVerify_Timeout(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Do not respond within the client's timeout. Return once the request is
		// cancelled, with a bounded fallback so Close() cannot block forever if
		// the cancellation is not observed server-side. The fallback stays well
		// above the 100ms client timeout set below so the client always times out
		// first, while keeping Close() short.
		//
		// [Ja] クライアントのタイムアウト内では応答しない。リクエストがキャンセル
		// されたら返し、サーバー側でキャンセルを観測できなくても Close() が無期限に
		// ブロックしないよう上限付きのフォールバックを設ける。フォールバックは下で
		// 設定する 100ms のクライアントタイムアウトより十分長くして必ずクライアント側が
		// 先にタイムアウトするようにしつつ、Close() を短く保つ。
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
		}
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}
	// Shorten the client timeout so the test does not wait the full requestTimeout.
	//
	// [Ja] テストが requestTimeout をフルに待たないよう、クライアントのタイムアウトを
	// 短くする。
	client.httpClient.Timeout = 100 * time.Millisecond

	success, err := client.Verify(context.Background(), "test-token")
	if err == nil {
		t.Error("Verify() error = nil, want timeout error")
	}
	if success {
		t.Errorf("Verify() success = %v, want false", success)
	}
}

// TestVerify_InvalidJSON verifies that a malformed response body yields an error.
//
// [Ja] TestVerify_InvalidJSON は不正なレスポンスボディで error が返ることを検証する。
func TestVerify_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": invalid json`))
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}

	success, err := client.Verify(context.Background(), "test-token")
	if err == nil {
		t.Error("Verify() error = nil, want JSON decode error")
	}
	if success {
		t.Errorf("Verify() success = %v, want false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "JSON デコードに失敗") {
		t.Errorf("Verify() error = %v, want it to contain %q", err, "JSON デコードに失敗")
	}
}

// TestVerify_NonOKStatusCode verifies that a non-200 response yields an error.
//
// [Ja] TestVerify_NonOKStatusCode は非 200 のレスポンスで error が返ることを検証する。
func TestVerify_NonOKStatusCode(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}

	success, err := client.Verify(context.Background(), "test-token")
	if err == nil {
		t.Error("Verify() error = nil, want HTTP error")
	}
	if success {
		t.Errorf("Verify() success = %v, want false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "siteverify API がエラーを返しました") {
		t.Errorf("Verify() error = %v, want it to contain %q", err, "siteverify API がエラーを返しました")
	}
}

// TestNewClient verifies that NewClient stores the secret key and sets the
// bounded HTTP client.
//
// [Ja] TestNewClient は NewClient がシークレットキーを保持し、期限付きの HTTP
// クライアントを設定することを検証する。
func TestNewClient(t *testing.T) {
	t.Parallel()

	client := NewClient("my-secret-key")

	if client.secretKey != "my-secret-key" {
		t.Errorf("client.secretKey = %q, want %q", client.secretKey, "my-secret-key")
	}
	if client.httpClient == nil {
		t.Fatal("client.httpClient is nil")
	}
	if client.httpClient.Timeout != requestTimeout {
		t.Errorf("client.httpClient.Timeout = %v, want %v", client.httpClient.Timeout, requestTimeout)
	}
}
