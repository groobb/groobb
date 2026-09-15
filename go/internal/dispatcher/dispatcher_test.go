package dispatcher

import (
	"context"
	"database/sql"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// RiverのSQLiteクライアントとJobInserterの互換性をコンパイル時に検査する。
var _ JobInserter = (*river.Client[*sql.Tx])(nil)

// mockJobInserterは最後に投入されたジョブを記録し、Enqueue* メソッドが
// Insertに渡す引数・オプションをテストで検証できるようにする。
type mockJobInserter struct {
	called bool
	args   river.JobArgs
	opts   *river.InsertOpts
}

func (m *mockJobInserter) Insert(_ context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	m.called = true
	m.args = args
	m.opts = opts
	return &rivertype.JobInsertResult{}, nil
}

// TestNewDispatcher_StoresInserterはNewDispatcherが与えたJobInserterを保持する
// ことを検証する。これはすべてのEnqueue* メソッドが委譲する先のinserterである。
func TestNewDispatcher_StoresInserter(t *testing.T) {
	t.Parallel()

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if d.client != mock {
		t.Error("NewDispatcherは与えたJobInserterを保持していません")
	}
}

// TestSendEmailConfirmationArgs_Kindはジョブ種別の文字列を固定する。これは
// Riverが永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestSendEmailConfirmationArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (SendEmailConfirmationArgs{}).Kind(); got != "send_email_confirmation" {
		t.Errorf("Kind() = %q、期待値 = %q", got, "send_email_confirmation")
	}
}

// TestSendEmailConfirmationArgs_InsertOptsはジョブ単位の既定値を検証し、一時的な
// 送信失敗が捨てられずリトライされることを確認する。
func TestSendEmailConfirmationArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (SendEmailConfirmationArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q、期待値 = %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d、期待値 = 5", opts.MaxAttempts)
	}
}

// TestEnqueueEmailConfirmationはdispatcherがRiverに正しいArgsと、MaxAttemptsを
// 載せたInsertOptsを渡すことを検証する (リトライ既定値を失わせるnil optsを渡さない)。
func TestEnqueueEmailConfirmation(t *testing.T) {
	t.Parallel()

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if err := d.EnqueueEmailConfirmation(context.Background(), "user@example.dev", "123456", "ja"); err != nil {
		t.Fatalf("EnqueueEmailConfirmation()のエラー = %v", err)
	}

	if !mock.called {
		t.Fatal("Insertが呼ばれていません")
	}
	args, ok := mock.args.(SendEmailConfirmationArgs)
	if !ok {
		t.Fatalf("argsの型 = %T、期待値 = SendEmailConfirmationArgs", mock.args)
	}
	if args.Email != "user@example.dev" {
		t.Errorf("args.Email = %q、期待値 = %q", args.Email, "user@example.dev")
	}
	if args.Code != "123456" {
		t.Errorf("args.Code = %q、期待値 = %q", args.Code, "123456")
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q、期待値 = %q", args.Locale, "ja")
	}
	if mock.opts == nil {
		t.Fatal("optsがnilです (InsertOptsの既定値が失われます)")
	}
	if mock.opts.MaxAttempts != 5 {
		t.Errorf("opts.MaxAttempts = %d、期待値 = 5", mock.opts.MaxAttempts)
	}
}

// TestSendPasswordResetArgs_Kindはジョブ種別の文字列を固定する。これはRiverが
// 永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestSendPasswordResetArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (SendPasswordResetArgs{}).Kind(); got != "send_password_reset" {
		t.Errorf("Kind() = %q、期待値 = %q", got, "send_password_reset")
	}
}

// TestSendPasswordResetArgs_InsertOptsはジョブ単位の既定値を検証し、一時的な
// 送信失敗が捨てられずリトライされることを確認する。
func TestSendPasswordResetArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (SendPasswordResetArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q、期待値 = %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d、期待値 = 5", opts.MaxAttempts)
	}
}

// TestEnqueuePasswordResetはdispatcherがRiverに正しいArgsと、MaxAttemptsを
// 載せたInsertOptsを渡すことを検証する (リトライ既定値を失わせるnil optsを渡さない)。
func TestEnqueuePasswordReset(t *testing.T) {
	t.Parallel()

	const resetURL = "https://groobb.example.dev/password/edit?token=abc"

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if err := d.EnqueuePasswordReset(context.Background(), "user@example.dev", resetURL, "ja"); err != nil {
		t.Fatalf("EnqueuePasswordReset()のエラー = %v", err)
	}

	if !mock.called {
		t.Fatal("Insertが呼ばれていません")
	}
	args, ok := mock.args.(SendPasswordResetArgs)
	if !ok {
		t.Fatalf("argsの型 = %T、期待値 = SendPasswordResetArgs", mock.args)
	}
	if args.Email != "user@example.dev" {
		t.Errorf("args.Email = %q、期待値 = %q", args.Email, "user@example.dev")
	}
	if args.ResetURL != resetURL {
		t.Errorf("args.ResetURL = %q、期待値 = %q", args.ResetURL, resetURL)
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q、期待値 = %q", args.Locale, "ja")
	}
	if mock.opts == nil {
		t.Fatal("optsがnilです (InsertOptsの既定値が失われます)")
	}
	if mock.opts.MaxAttempts != 5 {
		t.Errorf("opts.MaxAttempts = %d、期待値 = 5", mock.opts.MaxAttempts)
	}
}

// TestSendEmailChangeNotificationArgs_Kindはジョブ種別の文字列を固定する。これは
// Riverが永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestSendEmailChangeNotificationArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (SendEmailChangeNotificationArgs{}).Kind(); got != "send_email_change_notification" {
		t.Errorf("Kind() = %q、期待値 = %q", got, "send_email_change_notification")
	}
}

// TestSendEmailChangeNotificationArgs_InsertOptsはジョブ単位の既定値を検証し、
// 一時的な送信失敗が捨てられずリトライされることを確認する。
func TestSendEmailChangeNotificationArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (SendEmailChangeNotificationArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q、期待値 = %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d、期待値 = 5", opts.MaxAttempts)
	}
}

// TestPurgeWithdrawnUsersArgs_Kindはジョブ種別の文字列を固定する。これはRiverが
// 永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestPurgeWithdrawnUsersArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (PurgeWithdrawnUsersArgs{}).Kind(); got != "purge_withdrawn_users" {
		t.Errorf("Kind() = %q、期待値 = %q", got, "purge_withdrawn_users")
	}
}

// TestPurgeWithdrawnUsersArgs_InsertOptsはジョブ単位の既定値を検証する。パージは
// 冪等かつ定期実行のため、既定キューでメールジョブより少ない試行回数で走る。
func TestPurgeWithdrawnUsersArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (PurgeWithdrawnUsersArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q、期待値 = %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d、期待値 = 3", opts.MaxAttempts)
	}
}

// TestEnqueueEmailChangeNotificationはdispatcherがRiverに正しいArgsと、
// MaxAttemptsを載せたInsertOptsを渡すことを検証する (リトライ既定値を失わせるnil optsを
// 渡さない)。
func TestEnqueueEmailChangeNotification(t *testing.T) {
	t.Parallel()

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if err := d.EnqueueEmailChangeNotification(context.Background(), "old@example.dev", "new@example.dev", "ja"); err != nil {
		t.Fatalf("EnqueueEmailChangeNotification()のエラー = %v", err)
	}

	if !mock.called {
		t.Fatal("Insertが呼ばれていません")
	}
	args, ok := mock.args.(SendEmailChangeNotificationArgs)
	if !ok {
		t.Fatalf("argsの型 = %T、期待値 = SendEmailChangeNotificationArgs", mock.args)
	}
	if args.Email != "old@example.dev" {
		t.Errorf("args.Email = %q、期待値 = %q", args.Email, "old@example.dev")
	}
	if args.NewEmail != "new@example.dev" {
		t.Errorf("args.NewEmail = %q、期待値 = %q", args.NewEmail, "new@example.dev")
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q、期待値 = %q", args.Locale, "ja")
	}
	if mock.opts == nil {
		t.Fatal("optsがnilです (InsertOptsの既定値が失われます)")
	}
	if mock.opts.MaxAttempts != 5 {
		t.Errorf("opts.MaxAttempts = %d、期待値 = 5", mock.opts.MaxAttempts)
	}
}
