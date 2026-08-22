package testutil

import (
	"testing"

	"github.com/groobb/groobb/go/internal/config"
)

// TestContinuationTokenKey is the non-production HMAC key shared by test
// managers that issue and verify continuation tokens. It is deliberately a
// fixture rather than a runtime default.
//
// [Ja] TestContinuationTokenKey は continuation token を発行・検証するテスト用 Manager 間で
// 共有する非本番の HMAC 鍵です。実行時の既定値ではなく、意図的に fixture としています。
const TestContinuationTokenKey = "groobb-test-continuation-token-key-32-bytes"

// NewTestConfig returns the application configuration for tests that issue or
// verify continuation tokens, which is what the key it carries is for. A test
// whose collaborators never touch a continuation token does not need this: it
// builds the config it wants inline, and the extra key would only obscure which
// settings that test actually depends on.
//
// [Ja] NewTestConfig は continuation token を発行・検証するテスト向けにアプリケーションの
// 設定を返します。ここが持つ鍵はそのためのものです。collaborator が continuation token に
// 触れないテストはこれを必要とせず、必要な設定をその場で組み立てます。余分な鍵は、そのテスト
// が実際に何の設定に依存しているのかを曖昧にするだけであるためです。
func NewTestConfig(t testing.TB) *config.Config {
	t.Helper()

	return &config.Config{
		Env:                  "test",
		ContinuationTokenKey: TestContinuationTokenKey,
	}
}
