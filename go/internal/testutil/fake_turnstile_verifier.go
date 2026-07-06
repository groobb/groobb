package testutil

import "context"

// FakeTurnstileVerifier is a test double for turnstile.Verifier that returns
// canned results instead of calling the Cloudflare siteverify API, so handler
// tests can drive the pass / non-pass / verification-error paths without real
// HTTP. It records the last token it was asked to verify. It satisfies
// turnstile.Verifier structurally (matching the Verify signature), avoiding an
// import of the turnstile package here.
//
// [Ja] FakeTurnstileVerifier は turnstile.Verifier のテストダブルで、Cloudflare の
// siteverify API を呼ばずに定型の結果を返します。これによりハンドラーテストは実 HTTP
// なしで通過 / 非通過 / 検証エラーの各経路を検証できます。最後に検証を求められた
// トークンを記録します。(Verify シグネチャに一致して) turnstile.Verifier を構造的に
// 満たすため、ここで turnstile パッケージを import せずに済みます。
type FakeTurnstileVerifier struct {
	// Passed is the pass result Verify reports (true = the challenge passed).
	//
	// [Ja] Passed は Verify が返す通過結果です (true = チャレンジを通過)。
	Passed bool
	// Err, when non-nil, is returned by Verify to exercise the verification-error
	// path (a siteverify rejection or system failure).
	//
	// [Ja] Err は非 nil のとき、Verify が返す値です。検証エラーの経路 (siteverify の
	// 拒否やシステム障害) を検証できるようにします。
	Err error
	// Token records the last token passed to Verify, so a test can assert the
	// handler forwarded the submitted cf-turnstile-response field.
	//
	// [Ja] Token は最後に Verify へ渡されたトークンを記録します。ハンドラーが送信された
	// cf-turnstile-response フィールドを渡したことをテストが検証できるようにします。
	Token string
}

// Verify records the token and returns the canned Passed / Err result.
//
// [Ja] Verify はトークンを記録し、定型の Passed / Err の結果を返します。
func (f *FakeTurnstileVerifier) Verify(_ context.Context, token string) (bool, error) {
	f.Token = token
	return f.Passed, f.Err
}
