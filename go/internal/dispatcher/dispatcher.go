// Package dispatcher abstracts enqueueing background jobs onto the River job
// queue. Just as a Repository abstracts database access, the Dispatcher
// abstracts job-queue access: callers (UseCases) invoke Enqueue* methods
// without importing River or knowing the concrete job argument types.
//
// [Ja] dispatcher パッケージは、バックグラウンドジョブを River ジョブキューへ投入する
// 処理を抽象化する。Repository がデータベースアクセスを抽象化するのと同じ発想で、
// Dispatcher はジョブキューアクセスを抽象化する。呼び出し側 (UseCase) は River を
// import したり具体的なジョブ引数型を知ったりせずに Enqueue* メソッドを呼ぶ。
package dispatcher

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/groobb/groobb/go/internal/model"
)

// SendEmailConfirmationArgs are the arguments for the job that sends an email
// confirmation code. They are the primitive values the worker needs to render
// and send the mail, kept JSON-encodable for River to persist on the queue.
//
// Locale is one of those primitives rather than a model.Locale, here and in the
// other Args: the arguments are persisted as JSON, so the queue carries the
// language as text either way. The Enqueue* methods take a model.Locale and the
// workers convert what they read back, which keeps the type on both sides of the
// queue without asking River to encode it.
//
// [Ja] SendEmailConfirmationArgs はメール確認コードを送信するジョブの引数です。
// ワーカーがメールを描画・送信するのに必要なプリミティブ値で、River がキューに永続化
// できるよう JSON エンコード可能に保ちます。
//
// Locale が他の Args と同じく model.Locale ではなくプリミティブ値なのは、引数が JSON
// として永続化される以上、キューはいずれにせよ言語をテキストとして運ぶためです。
// Enqueue* メソッドが model.Locale を受け取り、ワーカーが読み出したものを変換すること
// で、River に型のエンコードを求めずにキューの両側で型を保てます。
type SendEmailConfirmationArgs struct {
	Email  string `json:"email"`
	Code   string `json:"code"`
	Locale string `json:"locale"`
}

// Kind returns the unique job identifier River uses to route the job to its
// worker.
//
// [Ja] Kind は River がジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (SendEmailConfirmationArgs) Kind() string { return "send_email_confirmation" }

// InsertOpts sets the per-job defaults: the default queue and up to 5 attempts,
// so a transient mail-send failure (e.g. a Resend hiccup) is retried rather than
// lost.
//
// [Ja] InsertOpts はジョブ単位の既定値を設定します。既定キューと最大 5 回の試行とし、
// 一時的なメール送信失敗 (例: Resend の不調) を失わずにリトライします。
func (SendEmailConfirmationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 5}
}

// SendPasswordResetArgs are the arguments for the job that sends a password reset
// mail. ResetURL is the absolute reset link (carrying the one-time token) the
// mail must present; Locale selects the language the mail is rendered in. They
// are kept JSON-encodable for River to persist on the queue.
//
// [Ja] SendPasswordResetArgs はパスワードリセットメールを送信するジョブの引数です。
// ResetURL はメールが提示すべき絶対リセットリンク (使い捨てトークンを含む)、Locale は
// メールを描画する言語を選びます。River がキューに永続化できるよう JSON エンコード可能に
// 保ちます。
type SendPasswordResetArgs struct {
	Email    string `json:"email"`
	ResetURL string `json:"reset_url"`
	Locale   string `json:"locale"`
}

// Kind returns the unique job identifier River uses to route the job to its
// worker.
//
// [Ja] Kind は River がジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (SendPasswordResetArgs) Kind() string { return "send_password_reset" }

// InsertOpts sets the per-job defaults: the default queue and up to 5 attempts,
// so a transient mail-send failure (e.g. a Resend hiccup) is retried rather than
// lost.
//
// [Ja] InsertOpts はジョブ単位の既定値を設定します。既定キューと最大 5 回の試行とし、
// 一時的なメール送信失敗 (例: Resend の不調) を失わずにリトライします。
func (SendPasswordResetArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 5}
}

// SendEmailChangeNotificationArgs are the arguments for the job that notifies a
// user's previous address that their account email was changed. Email is the
// recipient (the old address that just lost the account); NewEmail is the address
// the account was switched to, shown so the recipient can see where it went;
// Locale selects the language the mail is rendered in. They are kept
// JSON-encodable for River to persist on the queue.
//
// [Ja] SendEmailChangeNotificationArgs はユーザーの以前のアドレスに、アカウントの
// メールアドレスが変更されたことを通知するジョブの引数です。Email は宛先 (アカウントを
// 失ったばかりの旧アドレス)、NewEmail はアカウントの切り替え先アドレスで、宛先が変更先を
// 確認できるよう示します。Locale はメールを描画する言語を選びます。River がキューに
// 永続化できるよう JSON エンコード可能に保ちます。
type SendEmailChangeNotificationArgs struct {
	Email    string `json:"email"`
	NewEmail string `json:"new_email"`
	Locale   string `json:"locale"`
}

// Kind returns the unique job identifier River uses to route the job to its
// worker.
//
// [Ja] Kind は River がジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (SendEmailChangeNotificationArgs) Kind() string { return "send_email_change_notification" }

// InsertOpts sets the per-job defaults: the default queue and up to 5 attempts,
// so a transient mail-send failure (e.g. a Resend hiccup) is retried rather than
// lost.
//
// [Ja] InsertOpts はジョブ単位の既定値を設定します。既定キューと最大 5 回の試行とし、
// 一時的なメール送信失敗 (例: Resend の不調) を失わずにリトライします。
func (SendEmailChangeNotificationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 5}
}

// PurgeWithdrawnUsersArgs are the arguments for the job that physically deletes
// users whose withdrawal grace period has elapsed. The job needs no arguments (the
// cutoff is derived from the current time inside the UseCase), so the struct is
// empty; it exists to carry Kind and InsertOpts. Unlike the mail jobs there is no
// Enqueue* method: this job is not enqueued from a UseCase but registered as a
// River periodic job (see worker.NewClient), so River inserts it on a schedule.
//
// [Ja] PurgeWithdrawnUsersArgs は、退会の猶予期間を過ぎたユーザーを物理削除するジョブの
// 引数です。ジョブに引数は不要なため (cutoff は UseCase 内で現在時刻から導出する) 構造体は
// 空で、Kind と InsertOpts を運ぶために存在します。メールジョブと違い Enqueue* メソッドは
// ありません。このジョブは UseCase から投入するのではなく River の定期ジョブとして登録し
// (worker.NewClient を参照)、River がスケジュールに従って投入します。
type PurgeWithdrawnUsersArgs struct{}

// Kind returns the unique job identifier River uses to route the job to its
// worker.
//
// [Ja] Kind は River がジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (PurgeWithdrawnUsersArgs) Kind() string { return "purge_withdrawn_users" }

// InsertOpts sets the per-job defaults: the default queue and up to 3 attempts.
// Fewer retries than the mail jobs (5) are warranted because the purge is
// idempotent (DELETE ... WHERE deleted_at < cutoff) and periodic: if a run is
// exhausted, the next scheduled run catches up on everything still overdue.
//
// [Ja] InsertOpts はジョブ単位の既定値を設定します。既定キューと最大 3 回の試行とします。
// メールジョブ (5 回) より試行回数を少なくしているのは、パージが冪等
// (DELETE ... WHERE deleted_at < cutoff) かつ定期実行のためです。ある実行が試行を使い切っても、
// 次の定期実行が期限を過ぎた残りをまとめて処理します。
func (PurgeWithdrawnUsersArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 3}
}

// JobInserter is the single slice of the River client the Dispatcher depends
// on: inserting a job. *river.Client[*sql.Tx] satisfies this signature directly,
// so the worker client can be injected without a wrapper, and tests can pass a
// mock to assert which job and options were enqueued.
//
// [Ja] JobInserter は Dispatcher が依存する River クライアントの機能 (ジョブの投入) を
// 1 つだけ切り出したインターフェース。*river.Client[*sql.Tx] がこのシグネチャをそのまま
// 満たすため、ラッパーなしで worker クライアントを注入でき、テストではモックを渡して
// どのジョブ・オプションで投入されたかを検証できる。
type JobInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Dispatcher enqueues background jobs through a JobInserter. Concrete Enqueue*
// methods and their job argument types are added alongside each job in later
// tasks (the first being the email-confirmation job); this struct is the
// foundation those methods hang off.
//
// [Ja] Dispatcher は JobInserter を通じてバックグラウンドジョブを投入する。具体的な
// Enqueue* メソッドとそのジョブ引数型は、後続タスクで各ジョブと一緒に追加する (最初は
// メール確認ジョブ)。本構造体はそれらのメソッドが乗る土台である。
type Dispatcher struct {
	client JobInserter
}

// NewDispatcher builds a Dispatcher backed by the given JobInserter.
//
// [Ja] NewDispatcher は与えられた JobInserter を背後に持つ Dispatcher を生成する。
func NewDispatcher(client JobInserter) *Dispatcher {
	return &Dispatcher{client: client}
}

// EnqueueEmailConfirmation enqueues a job to send the confirmation code for
// email in locale. Callers (UseCases) pass primitive values; the Args struct and
// its options are assembled here so callers need not import River. The options
// come from the Args' own InsertOpts so the MaxAttempts default is applied
// (passing nil would drop it).
//
// [Ja] EnqueueEmailConfirmation は email 宛に locale で確認コードを送信するジョブを
// 投入します。呼び出し側 (UseCase) はプリミティブ値を渡し、Args 構造体とそのオプションは
// ここで組み立てるため、呼び出し側は River を import せずに済みます。オプションは Args
// 自身の InsertOpts から取り、MaxAttempts の既定値を適用します (nil を渡すと失われます)。
func (d *Dispatcher) EnqueueEmailConfirmation(ctx context.Context, email, code string, locale model.Locale) error {
	args := SendEmailConfirmationArgs{Email: email, Code: code, Locale: string(locale)}
	opts := args.InsertOpts()
	_, err := d.client.Insert(ctx, args, &opts)
	return err
}

// EnqueuePasswordReset enqueues a job to send the password reset mail to email,
// presenting resetURL, in locale. Like EnqueueEmailConfirmation it takes
// primitive values and assembles the Args and options here (from the Args' own
// InsertOpts so the MaxAttempts default is applied), so callers (UseCases) need
// not import River.
//
// [Ja] EnqueuePasswordReset は email 宛に resetURL を提示するパスワードリセットメールを
// locale で送信するジョブを投入します。EnqueueEmailConfirmation と同様にプリミティブ値を
// 取り、Args とオプションをここで (MaxAttempts の既定値が適用されるよう Args 自身の
// InsertOpts から) 組み立てるため、呼び出し側 (UseCase) は River を import せずに済みます。
func (d *Dispatcher) EnqueuePasswordReset(ctx context.Context, email, resetURL string, locale model.Locale) error {
	args := SendPasswordResetArgs{Email: email, ResetURL: resetURL, Locale: string(locale)}
	opts := args.InsertOpts()
	_, err := d.client.Insert(ctx, args, &opts)
	return err
}

// EnqueueEmailChangeNotification enqueues a job to notify email (the user's old
// address) that their account email was changed to newEmail, rendered in locale.
// Like the other Enqueue* methods it takes primitive values and assembles the
// Args and options here (from the Args' own InsertOpts so the MaxAttempts default
// is applied), so callers (UseCases) need not import River.
//
// [Ja] EnqueueEmailChangeNotification は email (ユーザーの旧アドレス) 宛に、アカウントの
// メールアドレスが newEmail に変更されたことを locale で通知するジョブを投入します。
// 他の Enqueue* メソッドと同様にプリミティブ値を取り、Args とオプションをここで
// (MaxAttempts の既定値が適用されるよう Args 自身の InsertOpts から) 組み立てるため、
// 呼び出し側 (UseCase) は River を import せずに済みます。
func (d *Dispatcher) EnqueueEmailChangeNotification(ctx context.Context, email, newEmail string, locale model.Locale) error {
	args := SendEmailChangeNotificationArgs{Email: email, NewEmail: newEmail, Locale: string(locale)}
	opts := args.InsertOpts()
	_, err := d.client.Insert(ctx, args, &opts)
	return err
}
