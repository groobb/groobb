package worker_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/worker"
)

// TestNewClient_UnopenableDatabasePathは、開けないパスに対してNewClientが
// 中途半端なクライアントを返さず即座に失敗することを検証する。
func TestNewClient_UnopenableDatabasePath(t *testing.T) {
	t.Parallel()

	// 親ディレクトリが存在しないため、SQLiteはファイルを作成できない。
	path := filepath.Join(t.TempDir(), "missing", "groobb.sqlite")

	client, err := worker.NewClient(context.Background(), path, &config.Config{})
	if err == nil {
		t.Fatal("開けないデータベースパスに対してエラーが返るべきです")
	}
	if client != nil {
		t.Error("エラー時はclientがnilであるべきです")
	}
}

// TestNewClientはテストDBに対してクライアントを構築・起動し、また片付ける。
// send_email_confirmationワーカーが登録されたことでStartが成功する (Riverは起動に
// 最低1つのワーカーを要求する)。ジョブは投入しないため何も処理されず、メールも送られ
// ない。Work経路自体はNoopSenderを使うTestSendEmailConfirmationWorker_Workが担う。
func TestNewClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	client, err := worker.NewClient(ctx, testutil.SetupDBPath(t), &config.Config{})
	if err != nil {
		t.Fatalf("NewClientに失敗: %v", err)
	}

	if client.Client() == nil {
		t.Fatal("Client() は基盤のRiverクライアントを返すべきです")
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Startに失敗: %v", err)
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stopに失敗: %v", err)
	}
}

// TestNewClient_SMTPProviderはSMTPのtransportを選択してクライアントを構築し、
// リレーに接続することなく設定が受け入れられ配線されることを確認する。
func TestNewClient_SMTPProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := &config.Config{
		EmailProvider: config.EmailProviderSMTP,
		SMTPHost:      "smtp.example.dev",
		SMTPPort:      587,
		EmailFrom:     "noreply@example.dev",
	}

	client, err := worker.NewClient(ctx, testutil.SetupDBPath(t), cfg)
	if err != nil {
		t.Fatalf("NewClientに失敗: %v", err)
	}
	t.Cleanup(func() { _ = client.Stop(ctx) })

	if client.Client() == nil {
		t.Fatal("Client() は基盤のRiverクライアントを返すべきです")
	}
}

// TestNewClient_UnknownEmailProviderは、使用できないメールプロバイダーが構築を
// 失敗させることを検証する。データベースパスも開けないものにしてあるため、返るエラーは
// 接続を開く前にsenderが構築されることも示す。
func TestNewClient_UnknownEmailProvider(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "groobb.sqlite")

	client, err := worker.NewClient(context.Background(), path, &config.Config{EmailProvider: "sendmail"})
	if err == nil {
		t.Fatal("未知のメールプロバイダーに対してエラーが返るべきです")
	}
	if client != nil {
		t.Error("エラー時はclientがnilであるべきです")
	}
	if !strings.Contains(err.Error(), "sendmail") {
		t.Errorf("エラーはメールプロバイダーの失敗を示すべきです: %v", err)
	}
}
