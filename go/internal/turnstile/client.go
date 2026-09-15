// turnstileパッケージはCloudflare TurnstileによるBot対策を提供する。
// Verifierインターフェースと、Cloudflareのsiteverify APIに対してレスポンス
// トークンを検証するHTTPクライアントを含む。
package turnstile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// siteverifyURLはCloudflare Turnstileのサーバー側検証エンドポイント。
	siteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// requestTimeoutはsiteverifyへのHTTP呼び出しに上限を設け、応答が
	// 返らないときに検証を待つハンドラーがブロックされないようにする。
	requestTimeout = 10 * time.Second
)

// VerifierはTurnstileのレスポンストークンを検証する。
type Verifier interface {
	// VerifyはtokenがTurnstileチャレンジを通過したかを返す。siteverifyに
	// 到達しない想定内の非通過 (空トークン) では (false, nil) を返す。検証拒否
	// (siteverifyのsuccess:false。error-codesはログ用に含める) とシステム障害
	// (ネットワーク / デコード / 非200) の双方で非nilのerrorを返す。呼び出し側は
	// (falseまたはerror) をいずれも非通過として扱う。
	Verify(ctx context.Context, token string) (bool, error)
}

// ClientはCloudflareのsiteverify APIに対してTurnstileトークンを
// 検証する。Verifierを実装する。
type Client struct {
	secretKey  string
	httpClient *http.Client
}

// VerifyResponseはCloudflare siteverify APIのレスポンス。
type VerifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

// verifyRequestはCloudflare siteverify APIへ送るリクエストボディ。
type verifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

// NewClientはsecretKeyでsiteverifyに認証するClientを構築する。
// 保持するのはシークレットキーのみ。サイトキーは検証に使わずconfigから
// テンプレートへ直接渡すため、Clientには持たせない。
func NewClient(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// Verifyはtokenをsiteverify APIに照合し、通過したかを返す。
func (c *Client) Verify(ctx context.Context, token string) (bool, error) {
	// シークレットキーが空なのはTurnstileが無効な状態 (configが両キーを空に
	// するdev / testの構成)。検証をバイパスし、無効時はすべてのリクエストを通す。
	if c.secretKey == "" {
		return true, nil
	}

	// トークンが空なのは、ウィジェットを解かずにフォームが送信されたケース
	// (JavaScriptのブロックやBotによる直接POST)。これはシステムエラーではなく
	// 想定内の非通過なので (false, nil) を返し、空POSTのたびにerrorを上げずに
	// ハンドラー側でwarnログへ寄せる。
	if token == "" {
		return false, nil
	}

	reqBody := verifyRequest{
		Secret:   c.secretKey,
		Response: token,
	}
	// Turnstile siteverify APIは仕様上リクエストボディにsecretを含める必要が
	// あるため、gosec G117 (secret形状のフィールドがシリアライズされる指摘) はここでは
	// false positiveであり抑制する。
	//nolint:gosec // G117
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("リクエストボディのJSONエンコードに失敗: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, siteverifyURL, bytes.NewReader(jsonBody))
	if err != nil {
		return false, fmt.Errorf("HTTPリクエストの作成に失敗: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("siteverifyへのリクエスト送信に失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("siteverifyレスポンスの読み込みに失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("siteverify APIがエラーを返しました (ステータスコード: %d): %s", resp.StatusCode, string(body))
	}

	var verifyResp VerifyResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return false, fmt.Errorf("siteverifyレスポンスのJSONデコードに失敗: %w", err)
	}

	if !verifyResp.Success {
		// error-codesがあれば含めて返し、ハンドラーがトークン拒否の理由を
		// ログに残せるようにする。呼び出し側からはどちらの分岐も同じ非通過。
		if len(verifyResp.ErrorCodes) > 0 {
			return false, fmt.Errorf("turnstile検証に失敗しました (エラーコード: %v)", verifyResp.ErrorCodes)
		}
		return false, fmt.Errorf("turnstile検証に失敗しました")
	}

	return true, nil
}
