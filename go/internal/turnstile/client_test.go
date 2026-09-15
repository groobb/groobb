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

// mockTransportはすべてのリクエストをtarget (httptestサーバー) に振り向け、
// ハードコードされたsiteverifyURL定数を変えずにClientをテストできるようにする。
type mockTransport struct {
	target string
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.target, "http://")
	req.URL.Path = ""
	return http.DefaultTransport.RoundTrip(req)
}

// blockedTransportはリクエストが試みられたらテストを失敗させる。バイパス経路
// (空シークレット・空トークン) はsiteverifyに接続せず返す必要がある。
type blockedTransport struct {
	t *testing.T
}

func (bt *blockedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	bt.t.Errorf("バイパスする経路でVerifyがsiteverifyに接続した (HTTPリクエストは発生すべきでない)")
	return nil, fmt.Errorf("想定外のリクエスト")
}

// TestVerify_Successは成功レスポンスで (true, nil) が返ることを検証する。
func TestVerify_Success(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q、期待値 = application/json", got)
		}

		// リクエストがsecretとトークンをsiteverifyの要求するJSONフィールド名で
		// 運んでいることを検証する。verifyRequestのタグがずれると本番APIでしか弾かれない
		// ため、ここで捕捉する。
		var reqBody verifyRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("リクエストボディのデコードに失敗: %v", err)
		}
		if reqBody.Secret != "test-secret-key" {
			t.Errorf("リクエストのsecret = %q、期待値 = %q", reqBody.Secret, "test-secret-key")
		}
		if reqBody.Response != "test-token" {
			t.Errorf("リクエストのresponse = %q、期待値 = %q", reqBody.Response, "test-token")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "challenge_ts": "2026-01-01T00:00:00Z", "hostname": "groobb.example.dev"}`))
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}

	success, err := client.Verify(context.Background(), "test-token")
	if err != nil {
		t.Errorf("Verify()のエラー = %v、期待値 = nil", err)
	}
	if !success {
		t.Errorf("Verify()のsuccess = %v、期待値 = true", success)
	}
}

// TestVerify_Failureはsuccess=falseのレスポンスでerrorが返ることを検証する。
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
		t.Error("Verify()のエラー = nil、エラーを期待")
	}
	if success {
		t.Errorf("Verify()のsuccess = %v、期待値 = false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "turnstile検証に失敗しました") {
		t.Errorf("Verify()のエラー = %v、%q を含むことを期待", err, "turnstile検証に失敗しました")
	}
}

// TestVerify_FailureWithErrorCodesはerror-codesが返り値のerrorに含まれる
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
		t.Error("Verify()のエラー = nil、エラーを期待")
	}
	if success {
		t.Errorf("Verify()のsuccess = %v、期待値 = false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "エラーコード") {
		t.Errorf("Verify()のエラー = %v、%q を含むことを期待", err, "エラーコード")
	}
}

// TestVerify_EmptyTokenは空トークンが想定内の非通過であることを検証する。
// siteverifyに接続せず (false, nil) を返す。
func TestVerify_EmptyToken(t *testing.T) {
	t.Parallel()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &blockedTransport{t: t}

	success, err := client.Verify(context.Background(), "")
	if err != nil {
		t.Errorf("Verify()のエラー = %v、期待値 = nil", err)
	}
	if success {
		t.Errorf("Verify()のsuccess = %v、期待値 = false", success)
	}
}

// TestVerify_EmptySecretBypassは空のシークレットキーが検証をバイパスする
// ことを検証する。siteverifyに接続せず (true, nil) を返す。
func TestVerify_EmptySecretBypass(t *testing.T) {
	t.Parallel()

	client := NewClient("")
	client.httpClient.Transport = &blockedTransport{t: t}

	success, err := client.Verify(context.Background(), "any-token")
	if err != nil {
		t.Errorf("Verify()のエラー = %v、期待値 = nil", err)
	}
	if !success {
		t.Errorf("Verify()のsuccess = %v、期待値 = true", success)
	}
}

// TestVerify_Timeoutは、siteverifyがクライアントのタイムアウト内に応答しない
// ときVerifyがerrorで失敗することを検証する。
func TestVerify_Timeout(t *testing.T) {
	t.Parallel()

	mockServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// クライアントのタイムアウト内では応答しない。リクエストがキャンセル
		// されたら返し、サーバー側でキャンセルを観測できなくてもClose() が無期限に
		// ブロックしないよう上限付きのフォールバックを設ける。フォールバックは下で
		// 設定する100msのクライアントタイムアウトより十分長くして必ずクライアント側が
		// 先にタイムアウトするようにしつつ、Close() を短く保つ。
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
		}
	}))
	defer mockServer.Close()

	client := NewClient("test-secret-key")
	client.httpClient.Transport = &mockTransport{target: mockServer.URL}
	// テストがrequestTimeoutをフルに待たないよう、クライアントのタイムアウトを
	// 短くする。
	client.httpClient.Timeout = 100 * time.Millisecond

	success, err := client.Verify(context.Background(), "test-token")
	if err == nil {
		t.Error("Verify()のエラー = nil、タイムアウトのエラーを期待")
	}
	if success {
		t.Errorf("Verify()のsuccess = %v、期待値 = false", success)
	}
}

// TestVerify_InvalidJSONは不正なレスポンスボディでerrorが返ることを検証する。
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
		t.Error("Verify()のエラー = nil、JSONデコードのエラーを期待")
	}
	if success {
		t.Errorf("Verify()のsuccess = %v、期待値 = false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "JSONデコードに失敗") {
		t.Errorf("Verify()のエラー = %v、%q を含むことを期待", err, "JSONデコードに失敗")
	}
}

// TestVerify_NonOKStatusCodeは非200のレスポンスでerrorが返ることを検証する。
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
		t.Error("Verify()のエラー = nil、HTTPのエラーを期待")
	}
	if success {
		t.Errorf("Verify()のsuccess = %v、期待値 = false", success)
	}
	if err != nil && !strings.Contains(err.Error(), "siteverify APIがエラーを返しました") {
		t.Errorf("Verify()のエラー = %v、%q を含むことを期待", err, "siteverify APIがエラーを返しました")
	}
}

// TestNewClientはNewClientがシークレットキーを保持し、期限付きのHTTP
// クライアントを設定することを検証する。
func TestNewClient(t *testing.T) {
	t.Parallel()

	client := NewClient("my-secret-key")

	if client.secretKey != "my-secret-key" {
		t.Errorf("client.secretKey = %q、期待値 = %q", client.secretKey, "my-secret-key")
	}
	if client.httpClient == nil {
		t.Fatal("client.httpClientがnilになっている")
	}
	if client.httpClient.Timeout != requestTimeout {
		t.Errorf("client.httpClient.Timeout = %v、期待値 = %v", client.httpClient.Timeout, requestTimeout)
	}
}
