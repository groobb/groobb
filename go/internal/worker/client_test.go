package worker_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
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

	client, err := worker.NewClient(context.Background(), "://not-a-valid-url", &config.Config{})
	if err == nil {
		t.Fatal("不正な databaseURL に対してエラーが返るべきです")
	}
	if client != nil {
		t.Error("エラー時は client が nil であるべきです")
	}
}

// TestNewClient builds the client against the test database, starts it, and
// tears it back down. Start now succeeds because the send_email_confirmation
// worker is registered (River requires at least one worker to start). No job is
// enqueued, so nothing is processed and no email is sent; the Work path itself is
// covered by TestSendEmailConfirmationWorker_Work with a NoopSender.
//
// [Ja] TestNewClient はテスト DB に対してクライアントを構築・起動し、また片付ける。
// send_email_confirmation ワーカーが登録されたことで Start が成功する (River は起動に
// 最低 1 つのワーカーを要求する)。ジョブは投入しないため何も処理されず、メールも送られ
// ない。Work 経路自体は NoopSender を使う TestSendEmailConfirmationWorker_Work が担う。
func TestNewClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	client, err := worker.NewClient(ctx, testutil.DatabaseURL(), &config.Config{})
	if err != nil {
		t.Fatalf("NewClient に失敗: %v", err)
	}

	if client.Client() == nil {
		t.Fatal("Client() は基盤の River クライアントを返すべきです")
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start に失敗: %v", err)
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stop に失敗: %v", err)
	}
}
