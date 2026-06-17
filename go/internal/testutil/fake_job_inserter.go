package testutil

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// FakeJobInserter is a test double for dispatcher.JobInserter that records the
// last enqueued job instead of touching a real River queue, so UseCase and
// handler tests can build a dispatcher.Dispatcher without a running River
// client. It satisfies dispatcher.JobInserter structurally (matching River's
// Insert signature), avoiding an import of the dispatcher package here.
//
// [Ja] FakeJobInserter は dispatcher.JobInserter のテストダブルで、実際の River
// キューに触れず最後に投入されたジョブを記録します。これにより UseCase / ハンドラーの
// テストは、稼働中の River クライアント無しで dispatcher.Dispatcher を構築できます。
// (River の Insert シグネチャに一致して) dispatcher.JobInserter を構造的に満たすため、
// ここで dispatcher パッケージを import せずに済みます。
type FakeJobInserter struct {
	Called bool
	Args   river.JobArgs
	Opts   *river.InsertOpts
	// Err, when non-nil, is returned by Insert instead of a successful result, so
	// a test can exercise the enqueue-failure path.
	//
	// [Ja] Err は非 nil のとき、Insert が成功結果の代わりに返す値です。テストが
	// enqueue 失敗の経路を検証できるようにします。
	Err error
}

// Insert records the call and its arguments, then returns Err when it is set or
// an empty successful result otherwise.
//
// [Ja] Insert は呼び出しとその引数を記録し、Err が設定されていればそれを、なければ
// 空の成功結果を返します。
func (f *FakeJobInserter) Insert(_ context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.Called = true
	f.Args = args
	f.Opts = opts
	if f.Err != nil {
		return nil, f.Err
	}
	return &rivertype.JobInsertResult{}, nil
}
