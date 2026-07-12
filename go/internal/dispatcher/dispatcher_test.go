package dispatcher

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// Compile-time proof that River's pgx client satisfies JobInserter, so main.go
// can wire dispatcher.NewDispatcher(workerClient.Client()) directly once the
// first enqueue-side consumer (a UseCase) exists. If River changes the Insert
// signature, this breaks the build here rather than at the (not-yet-written)
// wiring site.
//
// [Ja] River の pgx クライアントが JobInserter を満たすことのコンパイル時保証。これにより
// 最初の投入側の利用者 (UseCase) ができた時点で main.go が
// dispatcher.NewDispatcher(workerClient.Client()) をそのまま配線できる。River が Insert の
// シグネチャを変えた場合、(まだ書かれていない) 配線箇所ではなくここでビルドが壊れる。
var _ JobInserter = (*river.Client[pgx.Tx])(nil)

// mockJobInserter records the last enqueued job so tests can assert which args
// and options a future Enqueue* method passes to Insert.
//
// [Ja] mockJobInserter は最後に投入されたジョブを記録し、将来の Enqueue* メソッドが
// Insert に渡す引数・オプションをテストで検証できるようにする。
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

// TestNewDispatcher_StoresInserter verifies that NewDispatcher keeps the given
// JobInserter, which is the inserter every future Enqueue* method delegates to.
//
// [Ja] TestNewDispatcher_StoresInserter は NewDispatcher が与えた JobInserter を保持する
// ことを検証する。これは将来のすべての Enqueue* メソッドが委譲する先の inserter である。
func TestNewDispatcher_StoresInserter(t *testing.T) {
	t.Parallel()

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if d.client != mock {
		t.Error("NewDispatcher は与えた JobInserter を保持していません")
	}
}

// TestSendEmailConfirmationArgs_Kind pins the job kind string, since it is the
// stable contract River uses to route persisted jobs to their worker.
//
// [Ja] TestSendEmailConfirmationArgs_Kind はジョブ種別の文字列を固定する。これは
// River が永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestSendEmailConfirmationArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (SendEmailConfirmationArgs{}).Kind(); got != "send_email_confirmation" {
		t.Errorf("Kind() = %q, want %q", got, "send_email_confirmation")
	}
}

// TestSendEmailConfirmationArgs_InsertOpts verifies the per-job defaults so a
// transient send failure is retried rather than dropped.
//
// [Ja] TestSendEmailConfirmationArgs_InsertOpts はジョブ単位の既定値を検証し、一時的な
// 送信失敗が捨てられずリトライされることを確認する。
func TestSendEmailConfirmationArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (SendEmailConfirmationArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q, want %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", opts.MaxAttempts)
	}
}

// TestEnqueueEmailConfirmation verifies the dispatcher hands River the right Args
// and the InsertOpts carrying MaxAttempts (so it does not pass a nil opts that
// would drop the retry default).
//
// [Ja] TestEnqueueEmailConfirmation は dispatcher が River に正しい Args と、MaxAttempts を
// 載せた InsertOpts を渡すことを検証する (リトライ既定値を失わせる nil opts を渡さない)。
func TestEnqueueEmailConfirmation(t *testing.T) {
	t.Parallel()

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if err := d.EnqueueEmailConfirmation(context.Background(), "user@example.dev", "123456", "ja"); err != nil {
		t.Fatalf("EnqueueEmailConfirmation() error = %v", err)
	}

	if !mock.called {
		t.Fatal("Insert が呼ばれていません")
	}
	args, ok := mock.args.(SendEmailConfirmationArgs)
	if !ok {
		t.Fatalf("args の型 = %T, want SendEmailConfirmationArgs", mock.args)
	}
	if args.Email != "user@example.dev" {
		t.Errorf("args.Email = %q, want %q", args.Email, "user@example.dev")
	}
	if args.Code != "123456" {
		t.Errorf("args.Code = %q, want %q", args.Code, "123456")
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q, want %q", args.Locale, "ja")
	}
	if mock.opts == nil {
		t.Fatal("opts が nil です (InsertOpts の既定値が失われます)")
	}
	if mock.opts.MaxAttempts != 5 {
		t.Errorf("opts.MaxAttempts = %d, want 5", mock.opts.MaxAttempts)
	}
}

// TestSendPasswordResetArgs_Kind pins the job kind string, since it is the stable
// contract River uses to route persisted jobs to their worker.
//
// [Ja] TestSendPasswordResetArgs_Kind はジョブ種別の文字列を固定する。これは River が
// 永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestSendPasswordResetArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (SendPasswordResetArgs{}).Kind(); got != "send_password_reset" {
		t.Errorf("Kind() = %q, want %q", got, "send_password_reset")
	}
}

// TestSendPasswordResetArgs_InsertOpts verifies the per-job defaults so a
// transient send failure is retried rather than dropped.
//
// [Ja] TestSendPasswordResetArgs_InsertOpts はジョブ単位の既定値を検証し、一時的な
// 送信失敗が捨てられずリトライされることを確認する。
func TestSendPasswordResetArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (SendPasswordResetArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q, want %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", opts.MaxAttempts)
	}
}

// TestEnqueuePasswordReset verifies the dispatcher hands River the right Args and
// the InsertOpts carrying MaxAttempts (so it does not pass a nil opts that would
// drop the retry default).
//
// [Ja] TestEnqueuePasswordReset は dispatcher が River に正しい Args と、MaxAttempts を
// 載せた InsertOpts を渡すことを検証する (リトライ既定値を失わせる nil opts を渡さない)。
func TestEnqueuePasswordReset(t *testing.T) {
	t.Parallel()

	const resetURL = "https://groobb.example.dev/password/edit?token=abc"

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if err := d.EnqueuePasswordReset(context.Background(), "user@example.dev", resetURL, "ja"); err != nil {
		t.Fatalf("EnqueuePasswordReset() error = %v", err)
	}

	if !mock.called {
		t.Fatal("Insert が呼ばれていません")
	}
	args, ok := mock.args.(SendPasswordResetArgs)
	if !ok {
		t.Fatalf("args の型 = %T, want SendPasswordResetArgs", mock.args)
	}
	if args.Email != "user@example.dev" {
		t.Errorf("args.Email = %q, want %q", args.Email, "user@example.dev")
	}
	if args.ResetURL != resetURL {
		t.Errorf("args.ResetURL = %q, want %q", args.ResetURL, resetURL)
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q, want %q", args.Locale, "ja")
	}
	if mock.opts == nil {
		t.Fatal("opts が nil です (InsertOpts の既定値が失われます)")
	}
	if mock.opts.MaxAttempts != 5 {
		t.Errorf("opts.MaxAttempts = %d, want 5", mock.opts.MaxAttempts)
	}
}

// TestSendEmailChangeNotificationArgs_Kind pins the job kind string, since it is
// the stable contract River uses to route persisted jobs to their worker.
//
// [Ja] TestSendEmailChangeNotificationArgs_Kind はジョブ種別の文字列を固定する。これは
// River が永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestSendEmailChangeNotificationArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (SendEmailChangeNotificationArgs{}).Kind(); got != "send_email_change_notification" {
		t.Errorf("Kind() = %q, want %q", got, "send_email_change_notification")
	}
}

// TestSendEmailChangeNotificationArgs_InsertOpts verifies the per-job defaults so
// a transient send failure is retried rather than dropped.
//
// [Ja] TestSendEmailChangeNotificationArgs_InsertOpts はジョブ単位の既定値を検証し、
// 一時的な送信失敗が捨てられずリトライされることを確認する。
func TestSendEmailChangeNotificationArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (SendEmailChangeNotificationArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q, want %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", opts.MaxAttempts)
	}
}

// TestPurgeWithdrawnUsersArgs_Kind pins the job kind string, since it is the stable
// contract River uses to route persisted jobs to their worker.
//
// [Ja] TestPurgeWithdrawnUsersArgs_Kind はジョブ種別の文字列を固定する。これは River が
// 永続化済みジョブをワーカーに振り分けるための安定した契約だからである。
func TestPurgeWithdrawnUsersArgs_Kind(t *testing.T) {
	t.Parallel()

	if got := (PurgeWithdrawnUsersArgs{}).Kind(); got != "purge_withdrawn_users" {
		t.Errorf("Kind() = %q, want %q", got, "purge_withdrawn_users")
	}
}

// TestPurgeWithdrawnUsersArgs_InsertOpts verifies the per-job defaults: the purge
// runs on the default queue with fewer attempts than the mail jobs, since it is
// idempotent and periodic.
//
// [Ja] TestPurgeWithdrawnUsersArgs_InsertOpts はジョブ単位の既定値を検証する。パージは
// 冪等かつ定期実行のため、既定キューでメールジョブより少ない試行回数で走る。
func TestPurgeWithdrawnUsersArgs_InsertOpts(t *testing.T) {
	t.Parallel()

	opts := (PurgeWithdrawnUsersArgs{}).InsertOpts()
	if opts.Queue != river.QueueDefault {
		t.Errorf("Queue = %q, want %q", opts.Queue, river.QueueDefault)
	}
	if opts.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", opts.MaxAttempts)
	}
}

// TestEnqueueEmailChangeNotification verifies the dispatcher hands River the right
// Args and the InsertOpts carrying MaxAttempts (so it does not pass a nil opts
// that would drop the retry default).
//
// [Ja] TestEnqueueEmailChangeNotification は dispatcher が River に正しい Args と、
// MaxAttempts を載せた InsertOpts を渡すことを検証する (リトライ既定値を失わせる nil opts を
// 渡さない)。
func TestEnqueueEmailChangeNotification(t *testing.T) {
	t.Parallel()

	mock := &mockJobInserter{}
	d := NewDispatcher(mock)

	if err := d.EnqueueEmailChangeNotification(context.Background(), "old@example.dev", "new@example.dev", "ja"); err != nil {
		t.Fatalf("EnqueueEmailChangeNotification() error = %v", err)
	}

	if !mock.called {
		t.Fatal("Insert が呼ばれていません")
	}
	args, ok := mock.args.(SendEmailChangeNotificationArgs)
	if !ok {
		t.Fatalf("args の型 = %T, want SendEmailChangeNotificationArgs", mock.args)
	}
	if args.Email != "old@example.dev" {
		t.Errorf("args.Email = %q, want %q", args.Email, "old@example.dev")
	}
	if args.NewEmail != "new@example.dev" {
		t.Errorf("args.NewEmail = %q, want %q", args.NewEmail, "new@example.dev")
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q, want %q", args.Locale, "ja")
	}
	if mock.opts == nil {
		t.Fatal("opts が nil です (InsertOpts の既定値が失われます)")
	}
	if mock.opts.MaxAttempts != 5 {
		t.Errorf("opts.MaxAttempts = %d, want 5", mock.opts.MaxAttempts)
	}
}
