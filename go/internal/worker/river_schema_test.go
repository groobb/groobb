package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/testutil"
)

// jobRoundTripTimeout bounds how long the test waits for the inserted job to
// reach the worker. River fetches an available job promptly, so the wait is
// normally milliseconds; the bound only keeps a schema that stops jobs from
// being fetched at all from hanging the package until the go test timeout.
//
// [Ja] jobRoundTripTimeout は、投入したジョブがワーカーへ届くのを待つ上限です。River は
// 空きジョブをすぐに取得するため通常はミリ秒で終わります。この上限は、ジョブがそもそも
// 取得されなくなるスキーマになったときに、go test のタイムアウトまでパッケージが固まるのを
// 防ぐためだけのものです。
const jobRoundTripTimeout = 30 * time.Second

// workedJob is what recordingWorker reports back: the arguments River decoded
// plus the job attributes it scanned from the row. They are captured together
// because the point of the test is that every one of them survives the trip
// through the database.
//
// [Ja] workedJob は recordingWorker が報告する内容です。River がデコードした引数と、
// 行からスキャンしたジョブの属性をまとめて持ちます。これらを一緒に捉えるのは、そのすべてが
// データベースを往復しても保たれることこそがこのテストの主題だからです。
type workedJob struct {
	args        dispatcher.SendEmailConfirmationArgs
	kind        string
	queue       string
	maxAttempts int
	attempt     int
}

// recordingWorker stands in for the real send_email_confirmation worker: it
// records the job it is handed instead of sending mail, so the round trip stays
// inside the process and needs no mail configuration.
//
// [Ja] recordingWorker は本物の send_email_confirmation ワーカーの代わりを務めます。
// メールを送る代わりに渡されたジョブを記録するため、往復はプロセス内で完結し、メールの
// 設定も要りません。
type recordingWorker struct {
	river.WorkerDefaults[dispatcher.SendEmailConfirmationArgs]
	worked chan workedJob
}

func (w *recordingWorker) Work(_ context.Context, job *river.Job[dispatcher.SendEmailConfirmationArgs]) error {
	w.worked <- workedJob{
		args:        job.Args,
		kind:        job.Kind,
		queue:       job.Queue,
		maxAttempts: job.MaxAttempts,
		attempt:     job.Attempt,
	}
	return nil
}

// TestRiverSchemaRoundTripsAJob verifies that the River tables vendored into
// db/migrations carry a job from insertion to a worker with every value intact.
//
// The migration is a hand-vendored copy of the schema River's own migrator
// produces, so what it has to get right is the set of columns and types River's
// generated queries name. Nothing else in the suite checks that: the migration
// applies on its own, TestNewClient starts and stops the client without
// enqueueing anything, TestAppliedRiverMigrationVersionMatchesLibrary compares
// version numbers, and the Dispatcher tests use a mock inserter. A column the
// schema lacks or spells differently therefore only surfaces once a job actually
// moves through the queue.
//
// Asserting on every argument field and on the attributes River persists
// alongside them keeps the check on the values that came back, rather than on
// the insert merely returning.
//
// [Ja] TestRiverSchemaRoundTripsAJob は、db/migrations に取り込んだ River のテーブルが、
// ジョブを投入からワーカーまで値を保ったまま運ぶことを検証します。
//
// このマイグレーションは River 自身のマイグレータが生成するスキーマを手作業で取り込んだ
// 写しであり、外してはならないのは River の生成クエリが名指しするカラムの集合と型です。
// これを検査するものは他にありません。マイグレーションは単体では適用でき、TestNewClient は
// ジョブを投入せずにクライアントを起動・停止するだけ、
// TestAppliedRiverMigrationVersionMatchesLibrary はバージョン番号を比べるだけ、Dispatcher の
// テストはモックの inserter を使います。したがって、スキーマに無い・綴りの違うカラムは、
// 実際にジョブがキューを通ったときにはじめて現れます。
//
// 引数のすべてのフィールドと、River が併せて永続化する属性を検証するのは、投入が単に成功した
// ことではなく、返ってきた値そのものを確かめるためです。
func TestRiverSchemaRoundTripsAJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	db, err := database.Open(ctx, testutil.SetupDBPath(t))
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	worked := make(chan workedJob, 1)
	workers := river.NewWorkers()
	river.AddWorker(workers, &recordingWorker{worked: worked})

	client, err := river.NewClient(riversqlite.New(db.Writer), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}},
		Workers: workers,
	})
	if err != nil {
		t.Fatalf("failed to build the River client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("failed to start the River client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Stop(ctx); err != nil {
			t.Errorf("failed to stop the River client: %v", err)
		}
	})

	// The values are deliberately distinct from one another so that two fields
	// swapped anywhere along the trip show up as a mismatch rather than as an
	// accidental match.
	//
	// [Ja] 値どうしを意図的に別のものにしているのは、往復のどこかで 2 つのフィールドが
	// 入れ替わったときに、たまたま一致するのではなく不一致として現れるようにするため。
	args := dispatcher.SendEmailConfirmationArgs{
		Email:  "round-trip@example.com",
		Code:   "246813",
		Locale: "en",
	}
	opts := args.InsertOpts()
	if _, err := client.Insert(ctx, args, &opts); err != nil {
		t.Fatalf("failed to insert the job: %v", err)
	}

	select {
	case got := <-worked:
		if got.args != args {
			t.Errorf("job arguments = %+v, want %+v", got.args, args)
		}
		if got.kind != args.Kind() {
			t.Errorf("job kind = %q, want %q", got.kind, args.Kind())
		}
		if got.queue != opts.Queue {
			t.Errorf("job queue = %q, want %q", got.queue, opts.Queue)
		}
		if got.maxAttempts != opts.MaxAttempts {
			t.Errorf("job max attempts = %d, want %d", got.maxAttempts, opts.MaxAttempts)
		}
		if got.attempt != 1 {
			t.Errorf("job attempt = %d, want 1 on the first run", got.attempt)
		}
	case <-time.After(jobRoundTripTimeout):
		t.Fatalf("the inserted job did not reach the worker within %s", jobRoundTripTimeout)
	}
}
