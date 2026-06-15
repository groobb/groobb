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
