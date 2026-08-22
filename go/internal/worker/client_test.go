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

// TestNewClient_UnopenableDatabasePath verifies NewClient fails fast on a path
// it cannot open instead of returning a half-built client.
//
// [Ja] TestNewClient_UnopenableDatabasePath は、開けないパスに対して NewClient が
// 中途半端なクライアントを返さず即座に失敗することを検証する。
func TestNewClient_UnopenableDatabasePath(t *testing.T) {
	t.Parallel()

	// The parent directory does not exist, so SQLite cannot create the file.
	//
	// [Ja] 親ディレクトリが存在しないため、SQLite はファイルを作成できない。
	path := filepath.Join(t.TempDir(), "missing", "groobb.sqlite")

	client, err := worker.NewClient(context.Background(), path, &config.Config{})
	if err == nil {
		t.Fatal("開けないデータベースパスに対してエラーが返るべきです")
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

	client, err := worker.NewClient(ctx, testutil.SetupDBPath(t), &config.Config{})
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

// TestNewClient_SMTPProvider builds the client with the SMTP transport selected,
// confirming the configuration is accepted and wired without contacting a relay.
//
// [Ja] TestNewClient_SMTPProvider は SMTP の transport を選択してクライアントを構築し、
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
		t.Fatalf("NewClient に失敗: %v", err)
	}
	t.Cleanup(func() { _ = client.Stop(ctx) })

	if client.Client() == nil {
		t.Fatal("Client() は基盤の River クライアントを返すべきです")
	}
}

// TestNewClient_UnknownEmailProvider verifies an unusable email provider fails
// the build. The database path is unopenable too, so the reported error also
// shows the sender is built before any connection is opened.
//
// [Ja] TestNewClient_UnknownEmailProvider は、使用できないメールプロバイダーが構築を
// 失敗させることを検証する。データベースパスも開けないものにしてあるため、返るエラーは
// 接続を開く前に sender が構築されることも示す。
func TestNewClient_UnknownEmailProvider(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "groobb.sqlite")

	client, err := worker.NewClient(context.Background(), path, &config.Config{EmailProvider: "sendmail"})
	if err == nil {
		t.Fatal("未知のメールプロバイダーに対してエラーが返るべきです")
	}
	if client != nil {
		t.Error("エラー時は client が nil であるべきです")
	}
	if !strings.Contains(err.Error(), "sendmail") {
		t.Errorf("エラーはメールプロバイダーの失敗を示すべきです: %v", err)
	}
}
