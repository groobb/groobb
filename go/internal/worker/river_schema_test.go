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

// jobRoundTripTimeoutは、投入したジョブがワーカーへ届くのを待つ上限です。Riverは
// 空きジョブをすぐに取得するため通常はミリ秒で終わります。この上限は、ジョブがそもそも
// 取得されなくなるスキーマになったときに、go testのタイムアウトまでパッケージが固まるのを
// 防ぐためだけのものです。
const jobRoundTripTimeout = 30 * time.Second

// workedJobはrecordingWorkerが報告する内容です。Riverがデコードした引数と、
// 行からスキャンしたジョブの属性をまとめて持ちます。これらを一緒に捉えるのは、そのすべてが
// データベースを往復しても保たれることこそがこのテストの主題だからです。
type workedJob struct {
	args        dispatcher.SendEmailConfirmationArgs
	kind        string
	queue       string
	maxAttempts int
	attempt     int
}

// recordingWorkerは本物のsend_email_confirmationワーカーの代わりを務めます。
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

// TestRiverSchemaRoundTripsAJobは、db/migrationsに取り込んだRiverのテーブルが、
// ジョブを投入からワーカーまで値を保ったまま運ぶことを検証します。
//
// このマイグレーションはRiver自身のマイグレータが生成するスキーマを手作業で取り込んだ
// 写しであり、外してはならないのはRiverの生成クエリが名指しするカラムの集合と型です。
// これを検査するものは他にありません。マイグレーションは単体では適用でき、TestNewClientは
// ジョブを投入せずにクライアントを起動・停止するだけ、
// TestAppliedRiverMigrationVersionMatchesLibraryはバージョン番号を比べるだけ、Dispatcherの
// テストはモックのinserterを使います。したがって、スキーマに無い・綴りの違うカラムは、
// 実際にジョブがキューを通ったときにはじめて現れます。
//
// 引数のすべてのフィールドと、Riverが併せて永続化する属性を検証するのは、投入が単に成功した
// ことではなく、返ってきた値そのものを確かめるためです。
func TestRiverSchemaRoundTripsAJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	db, err := database.Open(ctx, testutil.SetupDBPath(t))
	if err != nil {
		t.Fatalf("データベースのオープンに失敗: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("データベースのクローズに失敗: %v", err)
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
		t.Fatalf("Riverクライアントの構築に失敗: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Riverクライアントの開始に失敗: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Stop(ctx); err != nil {
			t.Errorf("Riverクライアントの停止に失敗: %v", err)
		}
	})

	// 値どうしを意図的に別のものにしているのは、往復のどこかで2つのフィールドが
	// 入れ替わったときに、たまたま一致するのではなく不一致として現れるようにするため。
	args := dispatcher.SendEmailConfirmationArgs{
		Email:  "round-trip@example.com",
		Code:   "246813",
		Locale: "en",
	}
	opts := args.InsertOpts()
	if _, err := client.Insert(ctx, args, &opts); err != nil {
		t.Fatalf("ジョブの投入に失敗: %v", err)
	}

	select {
	case got := <-worked:
		if got.args != args {
			t.Errorf("ジョブの引数 = %+v、期待値 = %+v", got.args, args)
		}
		if got.kind != args.Kind() {
			t.Errorf("ジョブのkind = %q、期待値 = %q", got.kind, args.Kind())
		}
		if got.queue != opts.Queue {
			t.Errorf("ジョブのqueue = %q、期待値 = %q", got.queue, opts.Queue)
		}
		if got.maxAttempts != opts.MaxAttempts {
			t.Errorf("ジョブの最大試行回数 = %d、期待値 = %d", got.maxAttempts, opts.MaxAttempts)
		}
		if got.attempt != 1 {
			t.Errorf("ジョブの試行回数 = %d、期待値 = 1 (初回の実行)", got.attempt)
		}
	case <-time.After(jobRoundTripTimeout):
		t.Fatalf("投入したジョブが %s 以内にワーカーへ届かなかった", jobRoundTripTimeout)
	}
}
