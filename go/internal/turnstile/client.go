// Package turnstile provides Cloudflare Turnstile bot protection: a Verifier
// interface and an HTTP client that verifies response tokens against the
// Cloudflare siteverify API.
//
// [Ja] turnstile パッケージは Cloudflare Turnstile による Bot 対策を提供する。
// Verifier インターフェースと、Cloudflare の siteverify API に対してレスポンス
// トークンを検証する HTTP クライアントを含む。
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
	// siteverifyURL is the Cloudflare Turnstile server-side validation endpoint.
	//
	// [Ja] siteverifyURL は Cloudflare Turnstile のサーバー側検証エンドポイント。
	siteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// requestTimeout bounds the siteverify HTTP call so a hung response cannot
	// block the handler that awaits verification.
	//
	// [Ja] requestTimeout は siteverify への HTTP 呼び出しに上限を設け、応答が
	// 返らないときに検証を待つハンドラーがブロックされないようにする。
	requestTimeout = 10 * time.Second
)

// Verifier verifies a Turnstile response token.
//
// [Ja] Verifier は Turnstile のレスポンストークンを検証する。
type Verifier interface {
	// Verify reports whether token passed the Turnstile challenge. It returns
	// (false, nil) for an expected non-pass that never reaches siteverify (an
	// empty token). It returns a non-nil error both for a verification rejection
	// (siteverify success:false, whose error-codes are surfaced for logging) and
	// for a system failure (network / decoding / non-200). Callers treat any
	// (false result or error) as a non-pass.
	//
	// [Ja] Verify は token が Turnstile チャレンジを通過したかを返す。siteverify に
	// 到達しない想定内の非通過 (空トークン) では (false, nil) を返す。検証拒否
	// (siteverify の success:false。error-codes はログ用に含める) とシステム障害
	// (ネットワーク / デコード / 非 200) の双方で非 nil の error を返す。呼び出し側は
	// (false または error) をいずれも非通過として扱う。
	Verify(ctx context.Context, token string) (bool, error)
}

// Client verifies Turnstile tokens against the Cloudflare siteverify API. It
// implements Verifier.
//
// [Ja] Client は Cloudflare の siteverify API に対して Turnstile トークンを
// 検証する。Verifier を実装する。
type Client struct {
	secretKey  string
	httpClient *http.Client
}

// VerifyResponse is the Cloudflare siteverify API response.
//
// [Ja] VerifyResponse は Cloudflare siteverify API のレスポンス。
type VerifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

// verifyRequest is the request body sent to the Cloudflare siteverify API.
//
// [Ja] verifyRequest は Cloudflare siteverify API へ送るリクエストボディ。
type verifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

// NewClient builds a Client that authenticates to siteverify with secretKey.
// Only the secret key is held: the site key is not used for verification and is
// passed to templates from config directly, so the Client does not carry it.
//
// [Ja] NewClient は secretKey で siteverify に認証する Client を構築する。
// 保持するのはシークレットキーのみ。サイトキーは検証に使わず config から
// テンプレートへ直接渡すため、Client には持たせない。
func NewClient(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// Verify checks token against the siteverify API and reports whether it passed.
//
// [Ja] Verify は token を siteverify API に照合し、通過したかを返す。
func (c *Client) Verify(ctx context.Context, token string) (bool, error) {
	// An empty secret key means Turnstile is disabled (the dev / test setup
	// where config clears both keys). Bypass verification so the disabled path
	// lets every request through.
	//
	// [Ja] シークレットキーが空なのは Turnstile が無効な状態 (config が両キーを空に
	// する dev / test の構成)。検証をバイパスし、無効時はすべてのリクエストを通す。
	if c.secretKey == "" {
		return true, nil
	}

	// An empty token means the form was submitted without solving the widget
	// (JavaScript blocked, or a bot posting directly). This is an expected
	// non-pass, not a system error, so return (false, nil) and let the handler
	// log it at warn level rather than surfacing an error for every empty POST.
	//
	// [Ja] トークンが空なのは、ウィジェットを解かずにフォームが送信されたケース
	// (JavaScript のブロックや Bot による直接 POST)。これはシステムエラーではなく
	// 想定内の非通過なので (false, nil) を返し、空 POST のたびに error を上げずに
	// ハンドラー側で warn ログへ寄せる。
	if token == "" {
		return false, nil
	}

	reqBody := verifyRequest{
		Secret:   c.secretKey,
		Response: token,
	}
	// The Turnstile siteverify API requires the secret in the request body by
	// spec, so gosec G117 (a secret-shaped field being marshaled) is a false
	// positive here and is suppressed.
	//
	// [Ja] Turnstile siteverify API は仕様上リクエストボディに secret を含める必要が
	// あるため、gosec G117 (secret 形状のフィールドがシリアライズされる指摘) はここでは
	// false positive であり抑制する。
	//nolint:gosec // G117
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("リクエストボディの JSON エンコードに失敗: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, siteverifyURL, bytes.NewReader(jsonBody))
	if err != nil {
		return false, fmt.Errorf("HTTP リクエストの作成に失敗: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("siteverify へのリクエスト送信に失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("siteverify レスポンスの読み込みに失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("siteverify API がエラーを返しました (ステータスコード: %d): %s", resp.StatusCode, string(body))
	}

	var verifyResp VerifyResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return false, fmt.Errorf("siteverify レスポンスの JSON デコードに失敗: %w", err)
	}

	if !verifyResp.Success {
		// Surface error-codes when present so the handler can log why the token
		// was rejected; both branches still map to the same non-pass for callers.
		//
		// [Ja] error-codes があれば含めて返し、ハンドラーがトークン拒否の理由を
		// ログに残せるようにする。呼び出し側からはどちらの分岐も同じ非通過。
		if len(verifyResp.ErrorCodes) > 0 {
			return false, fmt.Errorf("turnstile 検証に失敗しました (エラーコード: %v)", verifyResp.ErrorCodes)
		}
		return false, fmt.Errorf("turnstile 検証に失敗しました")
	}

	return true, nil
}
