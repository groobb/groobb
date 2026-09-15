// dispatcherパッケージは、バックグラウンドジョブをRiverジョブキューへ投入する
// 処理を抽象化する。Repositoryがデータベースアクセスを抽象化するのと同じ発想で、
// Dispatcherはジョブキューアクセスを抽象化する。呼び出し側 (UseCase) はRiverを
// importしたり具体的なジョブ引数型を知ったりせずにEnqueue* メソッドを呼ぶ。
package dispatcher

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/groobb/groobb/go/internal/model"
)

// SendEmailConfirmationArgsはメール確認コードを送信するジョブの引数です。
// ワーカーがメールを描画・送信するのに必要なプリミティブ値で、Riverがキューに永続化
// できるようJSONエンコード可能に保ちます。
//
// Localeが他のArgsと同じくmodel.Localeではなくプリミティブ値なのは、引数がJSON
// として永続化される以上、キューはいずれにせよ言語をテキストとして運ぶためです。
// Enqueue* メソッドがmodel.Localeを受け取り、ワーカーが読み出したものを変換すること
// で、Riverに型のエンコードを求めずにキューの両側で型を保てます。
type SendEmailConfirmationArgs struct {
	Email  string `json:"email"`
	Code   string `json:"code"`
	Locale string `json:"locale"`
}

// KindはRiverがジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (SendEmailConfirmationArgs) Kind() string { return "send_email_confirmation" }

// InsertOptsはジョブ単位の既定値を設定します。既定キューと最大5回の試行とし、
// 一時的なメール送信失敗 (例: Resendの不調) を失わずにリトライします。
func (SendEmailConfirmationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 5}
}

// SendPasswordResetArgsはパスワードリセットメールを送信するジョブの引数です。
// ResetURLはメールが提示すべき絶対リセットリンク (使い捨てトークンを含む)、Localeは
// メールを描画する言語を選びます。Riverがキューに永続化できるようJSONエンコード可能に
// 保ちます。
type SendPasswordResetArgs struct {
	Email    string `json:"email"`
	ResetURL string `json:"reset_url"`
	Locale   string `json:"locale"`
}

// KindはRiverがジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (SendPasswordResetArgs) Kind() string { return "send_password_reset" }

// InsertOptsはジョブ単位の既定値を設定します。既定キューと最大5回の試行とし、
// 一時的なメール送信失敗 (例: Resendの不調) を失わずにリトライします。
func (SendPasswordResetArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 5}
}

// SendEmailChangeNotificationArgsはユーザーの以前のアドレスに、アカウントの
// メールアドレスが変更されたことを通知するジョブの引数です。Emailは宛先 (アカウントを
// 失ったばかりの旧アドレス)、NewEmailはアカウントの切り替え先アドレスで、宛先が変更先を
// 確認できるよう示します。Localeはメールを描画する言語を選びます。Riverがキューに
// 永続化できるようJSONエンコード可能に保ちます。
type SendEmailChangeNotificationArgs struct {
	Email    string `json:"email"`
	NewEmail string `json:"new_email"`
	Locale   string `json:"locale"`
}

// KindはRiverがジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (SendEmailChangeNotificationArgs) Kind() string { return "send_email_change_notification" }

// InsertOptsはジョブ単位の既定値を設定します。既定キューと最大5回の試行とし、
// 一時的なメール送信失敗 (例: Resendの不調) を失わずにリトライします。
func (SendEmailChangeNotificationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 5}
}

// PurgeWithdrawnUsersArgsは、退会の猶予期間を過ぎたユーザーを物理削除するジョブの
// 引数です。ジョブに引数は不要なため (cutoffはUseCase内で現在時刻から導出する) 構造体は
// 空で、KindとInsertOptsを運ぶために存在します。メールジョブと違いEnqueue* メソッドは
// ありません。このジョブはUseCaseから投入するのではなくRiverの定期ジョブとして登録し
// (worker.NewClientを参照)、Riverがスケジュールに従って投入します。
type PurgeWithdrawnUsersArgs struct{}

// KindはRiverがジョブをワーカーに振り分けるために使う一意なジョブ識別子を
// 返します。
func (PurgeWithdrawnUsersArgs) Kind() string { return "purge_withdrawn_users" }

// InsertOptsはジョブ単位の既定値を設定します。既定キューと最大3回の試行とします。
// メールジョブ (5回) より試行回数を少なくしているのは、パージが冪等
// (DELETE ... WHERE deleted_at < cutoff) かつ定期実行のためです。ある実行が試行を使い切っても、
// 次の定期実行が期限を過ぎた残りをまとめて処理します。
func (PurgeWithdrawnUsersArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: 3}
}

// JobInserterはDispatcherが依存するRiverクライアントの機能 (ジョブの投入) を
// 1つだけ切り出したインターフェース。*river.Client[*sql.Tx] がこのシグネチャをそのまま
// 満たすため、ラッパーなしでworkerクライアントを注入でき、テストではモックを渡して
// どのジョブ・オプションで投入されたかを検証できる。
type JobInserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// DispatcherはJobInserterを通じてバックグラウンドジョブを投入する。
type Dispatcher struct {
	client JobInserter
}

// NewDispatcherは与えられたJobInserterを背後に持つDispatcherを生成する。
func NewDispatcher(client JobInserter) *Dispatcher {
	return &Dispatcher{client: client}
}

// EnqueueEmailConfirmationはemail宛にlocaleで確認コードを送信するジョブを
// 投入します。呼び出し側 (UseCase) はプリミティブ値を渡し、Args構造体とそのオプションは
// ここで組み立てるため、呼び出し側はRiverをimportせずに済みます。オプションはArgs
// 自身のInsertOptsから取り、MaxAttemptsの既定値を適用します (nilを渡すと失われます)。
func (d *Dispatcher) EnqueueEmailConfirmation(ctx context.Context, email, code string, locale model.Locale) error {
	args := SendEmailConfirmationArgs{Email: email, Code: code, Locale: string(locale)}
	opts := args.InsertOpts()
	_, err := d.client.Insert(ctx, args, &opts)
	return err
}

// EnqueuePasswordResetはemail宛にresetURLを提示するパスワードリセットメールを
// localeで送信するジョブを投入します。EnqueueEmailConfirmationと同様にプリミティブ値を
// 取り、Argsとオプションをここで (MaxAttemptsの既定値が適用されるようArgs自身の
// InsertOptsから) 組み立てるため、呼び出し側 (UseCase) はRiverをimportせずに済みます。
func (d *Dispatcher) EnqueuePasswordReset(ctx context.Context, email, resetURL string, locale model.Locale) error {
	args := SendPasswordResetArgs{Email: email, ResetURL: resetURL, Locale: string(locale)}
	opts := args.InsertOpts()
	_, err := d.client.Insert(ctx, args, &opts)
	return err
}

// EnqueueEmailChangeNotificationはemail (ユーザーの旧アドレス) 宛に、アカウントの
// メールアドレスがnewEmailに変更されたことをlocaleで通知するジョブを投入します。
// 他のEnqueue* メソッドと同様にプリミティブ値を取り、Argsとオプションをここで
// (MaxAttemptsの既定値が適用されるようArgs自身のInsertOptsから) 組み立てるため、
// 呼び出し側 (UseCase) はRiverをimportせずに済みます。
func (d *Dispatcher) EnqueueEmailChangeNotification(ctx context.Context, email, newEmail string, locale model.Locale) error {
	args := SendEmailChangeNotificationArgs{Email: email, NewEmail: newEmail, Locale: string(locale)}
	opts := args.InsertOpts()
	_, err := d.client.Insert(ctx, args, &opts)
	return err
}
