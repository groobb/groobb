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
)

// JobInserter is the single slice of the River client the Dispatcher depends
// on: inserting a job. *river.Client[pgx.Tx] satisfies this signature directly,
// so the worker client can be injected without a wrapper, and tests can pass a
// mock to assert which job and options were enqueued.
//
// [Ja] JobInserter は Dispatcher が依存する River クライアントの機能 (ジョブの投入) を
// 1 つだけ切り出したインターフェース。*river.Client[pgx.Tx] がこのシグネチャをそのまま
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
