package testutil

import (
	"testing"

	"github.com/groobb/groobb/go/internal/config"
)

// TestContinuationTokenKeyはcontinuation tokenを発行・検証するテスト用Manager間で
// 共有する非本番のHMAC鍵です。実行時の既定値ではなく、意図的にfixtureとしています。
const TestContinuationTokenKey = "groobb-test-continuation-token-key-32-bytes"

// NewTestConfigはcontinuation tokenを発行・検証するテスト向けにアプリケーションの
// 設定を返します。ここが持つ鍵はそのためのものです。collaboratorがcontinuation tokenに
// 触れないテストはこれを必要とせず、必要な設定をその場で組み立てます。余分な鍵は、そのテスト
// が実際に何の設定に依存しているのかを曖昧にするだけであるためです。
func NewTestConfig(t testing.TB) *config.Config {
	t.Helper()

	return &config.Config{
		Env:                  "test",
		ContinuationTokenKey: TestContinuationTokenKey,
	}
}
