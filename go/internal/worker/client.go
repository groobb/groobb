// Package worker manages the lifecycle of the River background job client:
// building its connection pool, registering workers, and starting/stopping the
// client. A worker is a thin adapter that converts a job's arguments and calls a
// UseCase; worker implementations are registered here as jobs are introduced in
// later tasks (the first being the email-confirmation job).
//
// [Ja] worker パッケージは River バックグラウンドジョブクライアントのライフサイクル
// (接続プールの構築・ワーカーの登録・起動/停止) を管理する。ワーカーはジョブの引数を
// 変換して UseCase を呼ぶ薄い Adapter であり、その実装は後続タスクでジョブが導入されるのに
// 合わせてここに登録する (最初はメール確認ジョブ)。
package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/email"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/usecase"
)

// purgeWithdrawnUsersInterval is how often the purge job runs. Daily is frequent
// enough for a retention-window cleanup: the query deletes everything already past
// the cutoff, so the exact cadence only bounds how long a purge-eligible row lingers,
// not whether it is eventually removed.
//
// [Ja] purgeWithdrawnUsersInterval はパージジョブの実行間隔です。保持期間の
// クリーンアップには日次で十分です。クエリは cutoff を過ぎたものをすべて削除するため、
// 正確な周期はパージ対象の行が残る最長時間を決めるだけで、いずれ削除されるかどうかには
// 影響しません。
const purgeWithdrawnUsersInterval = 24 * time.Hour

// Client wraps the River client together with the database connection it owns,
// so the two can be started and shut down as a single unit.
//
// [Ja] Client は River クライアントと、それが所有するデータベース接続をまとめて保持し、
// 起動と停止を 1 つの単位として扱えるようにする。
type Client struct {
	riverClient *river.Client[*sql.Tx]
	db          *database.DB
}

// NewClient opens its own connection to the database file and builds the River
// client on it. The write connection is dedicated to background job processing,
// separate from the one serving requests, because River writes on its own
// schedule (looking for available jobs, holding the leader election) and a
// shared single-writer connection would queue the application's writes behind
// that traffic. Worker-only dependencies are built from cfg inside this function
// (rather than injected) so they stay encapsulated here and never leak into the
// rest of the DI graph; more worker registrations are added here as jobs are
// introduced in later tasks.
//
// [Ja] NewClient はデータベースファイルへの専用の接続を開き、その上に River
// クライアントを構築する。書き込み用コネクションをリクエスト処理用と分けるのは、River が
// 自身のスケジュールで (空きジョブの探索やリーダー選出のために) 書き込むためで、単一の
// ライターを共有するとアプリケーションの書き込みがその後ろに並んでしまう。ワーカー専用の
// 依存は (注入ではなく) 本関数内で cfg から構築し、ここに閉じ込めて DI グラフの他の部分へ
// 漏らさない。追加のワーカー登録は、後続タスクでジョブが導入されるのに合わせてここに加える。
func NewClient(ctx context.Context, databasePath string, cfg *config.Config) (*Client, error) {
	// Build the worker-only email dependencies from cfg and register the mail
	// workers (email confirmation, password reset, and email-change
	// notification). The sender is built before the database connection is opened
	// so a rejected email configuration fails without leaving a connection to
	// close. Constructing the senders here keeps the provider configuration out of
	// main.go's DI graph, where no request-path code needs it; all per-mail
	// senders share the one base sender.
	//
	// [Ja] ワーカー専用のメール依存を cfg から構築し、メールワーカー (メール確認・
	// パスワードリセット・メールアドレス変更通知) を登録する。sender をデータベース接続を
	// 開く前に構築するのは、メール設定が拒否された場合に閉じるべき接続を残さず失敗させる
	// ため。ここで sender を構築することで、リクエスト経路のコードが必要としない
	// プロバイダー設定を main.go の DI グラフから締め出す。メール種別ごとの各 sender は
	// 1 つの基盤 sender を共有する。
	emailSender, err := newEmailSender(cfg)
	if err != nil {
		return nil, err
	}

	db, err := database.Open(ctx, databasePath)
	if err != nil {
		return nil, fmt.Errorf("ワーカー用データベース接続の作成に失敗: %w", err)
	}

	confirmationSender := email.NewConfirmationSender(emailSender)
	sendEmailConfirmationUC := usecase.NewSendEmailConfirmationUsecase(confirmationSender)

	passwordResetSender := email.NewPasswordResetSender(emailSender)
	sendPasswordResetUC := usecase.NewSendPasswordResetUsecase(passwordResetSender)

	emailChangeNotificationSender := email.NewEmailChangeNotificationSender(emailSender)
	sendEmailChangeNotificationUC := usecase.NewSendEmailChangeNotificationUsecase(emailChangeNotificationSender)

	// Build the withdrawn-user purge UseCase over the worker's own connection.
	// Unlike the mail jobs it needs database access, so a repository is built here
	// from that connection; it is a worker-only dependency (only the periodic purge
	// job uses it) and so stays out of main.go's request-path DI graph.
	//
	// [Ja] 退会済みユーザーのパージ UseCase をワーカー自身の接続上に構築する。メールジョブと
	// 違い DB アクセスが必要なため、リポジトリをここでその接続から構築する。これは
	// ワーカー専用の依存 (定期パージジョブだけが使う) のため、main.go のリクエスト経路の
	// DI グラフには載せない。
	purgeUserRepo := repository.NewUserRepository(db)
	purgeWithdrawnUsersUC := usecase.NewPurgeWithdrawnUsersUsecase(purgeUserRepo)

	workers := river.NewWorkers()
	river.AddWorker(workers, NewSendEmailConfirmationWorker(sendEmailConfirmationUC))
	river.AddWorker(workers, NewSendPasswordResetWorker(sendPasswordResetUC))
	river.AddWorker(workers, NewSendEmailChangeNotificationWorker(sendEmailChangeNotificationUC))
	river.AddWorker(workers, NewPurgeWithdrawnUsersWorker(purgeWithdrawnUsersUC))

	// Register the purge job as a daily periodic job. The constructor returns the
	// Args together with their own InsertOpts, so the MaxAttempts default is applied
	// (returning nil opts would drop it). RunOnStart is left off (nil opts on the
	// periodic job): there is nothing urgent to purge at boot, so the job simply
	// waits for its first scheduled tick rather than running on every restart or
	// leader election.
	//
	// [Ja] パージジョブを日次の定期ジョブとして登録する。コンストラクタは Args を自身の
	// InsertOpts と一緒に返すため、MaxAttempts の既定値が適用される (nil の opts を返すと
	// 失われる)。RunOnStart は付けない (定期ジョブの opts は nil): 起動時に急いでパージする
	// ものは無いため、ジョブは再起動やリーダー選出のたびに走らず、最初のスケジュール刻みを
	// 待つだけにする。
	periodicJobs := []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(purgeWithdrawnUsersInterval),
			func() (river.JobArgs, *river.InsertOpts) {
				args := dispatcher.PurgeWithdrawnUsersArgs{}
				opts := args.InsertOpts()
				return args, &opts
			},
			nil,
		),
	}

	// Logger: slog.Default() routes River's own job-execution and retry logging
	// through the structured logger, so observability is in place before any
	// worker is added (a worker's Work method then only needs to return errors).
	//
	// [Ja] Logger: slog.Default() により River 自身のジョブ実行・リトライログを構造化
	// ロガー経由で出力する。これでワーカー追加前から観測性が確保され、ワーカーの Work
	// メソッドはエラーを返すだけでよくなる。
	riverClient, err := river.NewClient(riversqlite.New(db.Writer), &river.Config{
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}},
		Workers:      workers,
		Logger:       slog.Default(),
		PeriodicJobs: periodicJobs,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ワーカーの River クライアントの作成に失敗: %w", err)
	}

	return &Client{
		riverClient: riverClient,
		db:          db,
	}, nil
}

// Start begins fetching and working jobs.
//
// [Ja] Start はジョブの取得と処理を開始する。
func (c *Client) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "River クライアントを起動します")
	return c.riverClient.Start(ctx)
}

// Stop drains in-flight jobs, stops the client, and closes the database
// connection it owns.
//
// [Ja] Stop は実行中のジョブをドレインしてクライアントを停止し、所有するデータベース
// 接続を閉じる。
func (c *Client) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "River クライアントを停止します")
	if err := c.riverClient.Stop(ctx); err != nil {
		return err
	}
	return c.db.Close()
}

// Client exposes the underlying River client so it can be wired into the
// Dispatcher (it satisfies dispatcher.JobInserter) for enqueueing jobs.
//
// [Ja] Client は基盤の River クライアントを公開し、ジョブ投入のため Dispatcher に
// 配線できるようにする (dispatcher.JobInserter を満たす)。
func (c *Client) Client() *river.Client[*sql.Tx] {
	return c.riverClient
}

// newEmailSender builds the base email sender the per-mail senders share,
// picking the transport named by the configuration. An unset provider resolves
// to Resend so that a Config built without one (a test, or a deployment made
// before the setting existed) keeps its previous transport.
//
// [Ja] newEmailSender はメール種別ごとの Sender が共有する基盤 Sender を構築し、設定が
// 指定する transport を選ぶ。プロバイダー未設定は Resend に解決する。これにより、設定を
// 持たずに構築された Config (テストや、この設定が存在する前のデプロイ) は従来の transport を
// 保つ。
func newEmailSender(cfg *config.Config) (email.Sender, error) {
	switch cfg.EmailProvider {
	case config.EmailProviderSMTP:
		return email.NewSMTPSender(email.SMTPConfig{
			Host:      cfg.SMTPHost,
			Port:      cfg.SMTPPort,
			Username:  cfg.SMTPUsername,
			Password:  cfg.SMTPPassword,
			TLSMode:   email.SMTPTLSMode(cfg.SMTPTLSMode),
			FromEmail: cfg.EmailFrom,
			FromName:  cfg.EmailFromName,
		}), nil
	case config.EmailProviderResend, "":
		return email.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom, cfg.EmailFromName), nil
	default:
		return nil, fmt.Errorf("未知のメール送信プロバイダーです: %q", cfg.EmailProvider)
	}
}
