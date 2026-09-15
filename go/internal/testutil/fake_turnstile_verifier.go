package testutil

import "context"

// FakeTurnstileVerifierはturnstile.Verifierのテストダブルで、Cloudflareの
// siteverify APIを呼ばずに定型の結果を返します。これによりハンドラーテストは実HTTP
// なしで通過 / 非通過 / 検証エラーの各経路を検証できます。最後に検証を求められた
// トークンを記録します。(Verifyシグネチャに一致して) turnstile.Verifierを構造的に
// 満たすため、ここでturnstileパッケージをimportせずに済みます。
type FakeTurnstileVerifier struct {
	// PassedはVerifyが返す通過結果です (true = チャレンジを通過)。
	Passed bool
	// Errは非nilのとき、Verifyが返す値です。検証エラーの経路 (siteverifyの
	// 拒否やシステム障害) を検証できるようにします。
	Err error
	// Tokenは最後にVerifyへ渡されたトークンを記録します。ハンドラーが送信された
	// cf-turnstile-responseフィールドを渡したことをテストが検証できるようにします。
	Token string
}

// Verifyはトークンを記録し、定型のPassed / Errの結果を返します。
func (f *FakeTurnstileVerifier) Verify(_ context.Context, token string) (bool, error) {
	f.Token = token
	return f.Passed, f.Err
}
