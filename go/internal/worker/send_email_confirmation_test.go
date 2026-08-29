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

// TestSendEmailConfirmationWorker_Work drives the worker's job-processing path
// (Args -> UseCase -> Sender) with a NoopSender, so it exercises the whole chain
// without River or a real Resend call. The full River Start -> fetch -> Work loop
// is not driven here because NewClient builds a real ResendSender internally,
// which would attempt a network send; this test covers the Work adapter, and the
// client test covers Start/Stop lifecycle.
//
// [Ja] TestSendEmailConfirmationWorker_Work は NoopSender でワーカーの処理経路
// (Args -> UseCase -> Sender) を駆動し、River や実際の Resend 呼び出し無しに全体の連鎖を
// 検証する。River の Start -> 取得 -> Work ループ全体はここでは駆動しない。NewClient は
// 内部で実 ResendSender を構築し、ネットワーク送信を試みてしまうためである。本テストは
// Work アダプタを、クライアントテストが Start/Stop のライフサイクルをそれぞれ担う。
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
		t.Fatalf("Work() error = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d, want 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.To != "user@example.dev" {
		t.Errorf("To = %q, want %q", sent.To, "user@example.dev")
	}
	if sent.Subject != "[Groobb] 確認用コード" {
		t.Errorf("Subject = %q, want %q", sent.Subject, "[Groobb] 確認用コード")
	}

	var sb strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &sb); err != nil {
		t.Fatalf("HTMLBody.Render() error = %v", err)
	}
	if !strings.Contains(sb.String(), "135790") {
		t.Error("HTML body missing the code")
	}
}

// TestSendEmailConfirmationWorker_Work_LocaleNamingNoDisplayLanguage drives the
// worker with a locale argument that names no display language, which is what a
// job written by an earlier build can hold, and verifies the mail is still sent,
// rendered in the default locale rather than in whatever the argument said.
//
// [Ja] TestSendEmailConfirmationWorker_Work_LocaleNamingNoDisplayLanguage は、
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
		t.Fatalf("Work() error = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d, want 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.Subject != "[Groobb] 確認用コード" {
		t.Errorf("Subject = %q, want the default locale's subject %q", sent.Subject, "[Groobb] 確認用コード")
	}

	var html strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &html); err != nil {
		t.Fatalf("HTMLBody.Render() error = %v", err)
	}
	htmlBody := html.String()
	if !strings.Contains(htmlBody, `lang="ja"`) {
		t.Errorf("HTML body missing the default locale's lang attribute: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "下記のコードを入力して") {
		t.Errorf("HTML body missing the default locale's content: %s", htmlBody)
	}

	var text strings.Builder
	if err := sent.TextBody.Render(context.Background(), &text); err != nil {
		t.Fatalf("TextBody.Render() error = %v", err)
	}
	if body := text.String(); !strings.Contains(body, "下記のコードを入力して") {
		t.Errorf("text body missing the default locale's content: %s", body)
	}
}

// TestSendEmailConfirmationWorker_Work_LocaleFallbackLogging drives the worker
// with each kind of locale argument and checks the fallback to the default
// locale is logged, and only then. The job succeeds and the mail is sent either
// way, so the log line is the only thing telling an operator that a queued job
// still carried a language this build does not know.
//
// It does not call t.Parallel: it swaps the process-wide default slog handler to
// capture the output, the same way the config tests do.
//
// [Ja] TestSendEmailConfirmationWorker_Work_LocaleFallbackLogging は各種のロケール引数
// でワーカーを駆動し、既定ロケールへのフォールバックがログに残ること、そしてそのときだけ
// 残ることを確認する。いずれの場合もジョブは成功しメールは送られるため、キューに残った
// ジョブがこのビルドの知らない言語を運んでいたことを運用者に伝えるのはログ行だけである。
//
// t.Parallel を呼ばないのは、出力を捕捉するためにプロセス全体のデフォルト slog ハンドラーを
// 差し替えるためで、config のテストと同じやり方である。
func TestSendEmailConfirmationWorker_Work_LocaleFallbackLogging(t *testing.T) {
	tests := []struct {
		name     string
		locale   string
		wantWarn bool
	}{
		{name: "a locale naming no display language warns", locale: "fr", wantWarn: true},
		{name: "a display language does not warn", locale: "ja", wantWarn: false},
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
				t.Fatalf("Work() error = %v", err)
			}

			logged := buf.String()
			if warned := strings.Contains(logged, "level=WARN"); warned != tt.wantWarn {
				t.Errorf("warned = %v, want %v (log: %s)", warned, tt.wantWarn, logged)
			}
			if tt.wantWarn && !strings.Contains(logged, "locale="+tt.locale) {
				t.Errorf("log missing the locale the job carried: %s", logged)
			}
		})
	}
}
