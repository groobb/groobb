package worker_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/worker"
)

// TestNewClient_InvalidDatabaseURL verifies NewClient fails fast on an
// unparseable connection string instead of returning a half-built client.
//
// [Ja] TestNewClient_InvalidDatabaseURL は、解析できない接続文字列に対して NewClient が
// 中途半端なクライアントを返さず即座に失敗することを検証する。
func TestNewClient_InvalidDatabaseURL(t *testing.T) {
	t.Parallel()

	client, err := worker.NewClient(context.Background(), "://not-a-valid-url")
	if err == nil {
		t.Fatal("不正な databaseURL に対してエラーが返るべきです")
	}
	if client != nil {
		t.Error("エラー時は client が nil であるべきです")
	}
}

// TestNewClient builds the client against the test database and tears it back
// down, proving the pool and River client wire up correctly and that Stop
// cleanly releases them. The full Start -> work -> Stop path is not exercised
// here because River's Start requires at least one registered worker, and the
// first worker is added in a later task (the email-confirmation job); that
// task's integration test covers Start.
//
// [Ja] TestNewClient はテスト DB に対してクライアントを構築し、また片付ける。これにより
// プールと River クライアントが正しく配線され、Stop がそれらをきれいに解放することを
// 確認する。River の Start は最低 1 つのワーカー登録を要求し、最初のワーカーは後続タスク
// (メール確認ジョブ) で追加されるため、Start -> 処理 -> Stop の全体経路はここでは検証
// しない (Start はそのタスクの統合テストでカバーする)。
func TestNewClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	client, err := worker.NewClient(ctx, testutil.DatabaseURL())
	if err != nil {
		t.Fatalf("NewClient に失敗: %v", err)
	}

	if client.Client() == nil {
		t.Fatal("Client() は基盤の River クライアントを返すべきです")
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stop に失敗: %v", err)
	}
}
