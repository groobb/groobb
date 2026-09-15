package worker_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/email"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/worker"
)

// TestSendEmailConfirmationWorker_WorkはNoopSenderでワーカーの処理経路
// (Args -> UseCase -> Sender) を駆動し、Riverや実際のResend呼び出し無しに全体の連鎖を
// 検証する。RiverのStart -> 取得 -> Workループ全体はここでは駆動しない。NewClientは
// 内部で実ResendSenderを構築し、ネットワーク送信を試みてしまうためである。本テストは
// Workアダプタを、クライアントテストがStart/Stopのライフサイクルをそれぞれ担う。
func TestSendEmailConfirmationWorker_Work(t *testing.T) {
	t.Parallel()

	noop := email.NewNoopSender()
	uc := usecase.NewSendEmailConfirmationUsecase(email.NewConfirmationSender(noop))
	w := worker.NewSendEmailConfirmationWorker(uc)

	job := &river.Job[dispatcher.SendEmailConfirmationArgs]{
		Args: dispatcher.SendEmailConfirmationArgs{
			Email:  "user@example.dev",
			Code:   "135790",
			Locale: "ja",
		},
	}

	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.To != "user@example.dev" {
		t.Errorf("To = %q、期待値 = %q", sent.To, "user@example.dev")
	}
	if sent.Subject != "[Groobb] 確認用コード" {
		t.Errorf("Subject = %q、期待値 = %q", sent.Subject, "[Groobb] 確認用コード")
	}

	var sb strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &sb); err != nil {
		t.Fatalf("HTMLBody.Render()のエラー = %v", err)
	}
	if !strings.Contains(sb.String(), "135790") {
		t.Error("HTML本文にコードが含まれていない")
	}
}

// TestSendEmailConfirmationWorker_Work_LocaleNamingNoDisplayLanguageは、
// 表示言語を名指さないロケール引数 (古いビルドが書いたジョブが持ちうる値) でワーカーを
// 駆動し、引数が名乗ったものではなく既定のロケールで描画されたメールが、それでも送信
// されることを検証する。
func TestSendEmailConfirmationWorker_Work_LocaleNamingNoDisplayLanguage(t *testing.T) {
	t.Parallel()

	noop := email.NewNoopSender()
	uc := usecase.NewSendEmailConfirmationUsecase(email.NewConfirmationSender(noop))
	w := worker.NewSendEmailConfirmationWorker(uc)

	job := &river.Job[dispatcher.SendEmailConfirmationArgs]{
		Args: dispatcher.SendEmailConfirmationArgs{
			Email:  "user@example.dev",
			Code:   "135790",
			Locale: "fr",
		},
	}

	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.Subject != "[Groobb] 確認用コード" {
		t.Errorf("Subject = %q、期待値は既定のロケールの件名 %q", sent.Subject, "[Groobb] 確認用コード")
	}

	var html strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &html); err != nil {
		t.Fatalf("HTMLBody.Render()のエラー = %v", err)
	}
	htmlBody := html.String()
	if !strings.Contains(htmlBody, `lang="ja"`) {
		t.Errorf("HTML本文に既定のロケールのlang属性が含まれていない: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "下記のコードを入力して") {
		t.Errorf("HTML本文に既定のロケールの文面が含まれていない: %s", htmlBody)
	}

	var text strings.Builder
	if err := sent.TextBody.Render(context.Background(), &text); err != nil {
		t.Fatalf("TextBody.Render()のエラー = %v", err)
	}
	if body := text.String(); !strings.Contains(body, "下記のコードを入力して") {
		t.Errorf("テキスト本文に既定のロケールの文面が含まれていない: %s", body)
	}
}

// TestSendEmailConfirmationWorker_Work_LocaleFallbackLoggingは各種のロケール引数
// でワーカーを駆動し、既定ロケールへのフォールバックがログに残ること、そしてそのときだけ
// 残ることを確認する。いずれの場合もジョブは成功しメールは送られるため、キューに残った
// ジョブがこのビルドの知らない言語を運んでいたことを運用者に伝えるのはログ行だけである。
//
// t.Parallelを呼ばないのは、出力を捕捉するためにプロセス全体のデフォルトslogハンドラーを
// 差し替えるためで、configのテストと同じやり方である。
func TestSendEmailConfirmationWorker_Work_LocaleFallbackLogging(t *testing.T) {
	tests := []struct {
		name     string
		locale   string
		wantWarn bool
	}{
		{name: "どの表示言語も指さないロケールは警告する", locale: "fr", wantWarn: true},
		{name: "表示言語は警告しない", locale: "ja", wantWarn: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(original) })

			noop := email.NewNoopSender()
			uc := usecase.NewSendEmailConfirmationUsecase(email.NewConfirmationSender(noop))
			w := worker.NewSendEmailConfirmationWorker(uc)

			job := &river.Job[dispatcher.SendEmailConfirmationArgs]{
				Args: dispatcher.SendEmailConfirmationArgs{
					Email:  "user@example.dev",
					Code:   "135790",
					Locale: tt.locale,
				},
			}

			if err := w.Work(context.Background(), job); err != nil {
				t.Fatalf("Work()のエラー = %v", err)
			}

			logged := buf.String()
			if warned := strings.Contains(logged, "level=WARN"); warned != tt.wantWarn {
				t.Errorf("warned = %v、期待値 = %v (log: %s)", warned, tt.wantWarn, logged)
			}
			if tt.wantWarn && !strings.Contains(logged, "locale="+tt.locale) {
				t.Errorf("ログにジョブが持っていたロケールが含まれていない: %s", logged)
			}
		})
	}
}
