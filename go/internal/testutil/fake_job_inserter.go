package testutil

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// FakeJobInserterはdispatcher.JobInserterのテストダブルで、実際のRiver
// キューに触れず最後に投入されたジョブを記録します。これによりUseCase / ハンドラーの
// テストは、稼働中のRiverクライアント無しでdispatcher.Dispatcherを構築できます。
// (RiverのInsertシグネチャに一致して) dispatcher.JobInserterを構造的に満たすため、
// ここでdispatcherパッケージをimportせずに済みます。
type FakeJobInserter struct {
	Called bool
	Args   river.JobArgs
	Opts   *river.InsertOpts
	// Errは非nilのとき、Insertが成功結果の代わりに返す値です。テストが
	// enqueue失敗の経路を検証できるようにします。
	Err error
}

// Insertは呼び出しとその引数を記録し、Errが設定されていればそれを、なければ
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
