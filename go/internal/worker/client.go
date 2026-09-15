// workerパッケージはRiverバックグラウンドジョブクライアントのライフサイクル
// (接続プールの構築・ワーカーの登録・起動/停止) を管理する。ワーカーはジョブの引数を
// 変換してUseCaseを呼ぶ薄いAdapterであり、NewClientで登録する。
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

// purgeWithdrawnUsersIntervalはパージジョブの実行間隔です。保持期間の
// クリーンアップには日次で十分です。クエリはcutoffを過ぎたものをすべて削除するため、
// 正確な周期はパージ対象の行が残る最長時間を決めるだけで、いずれ削除されるかどうかには
// 影響しません。
const purgeWithdrawnUsersInterval = 24 * time.Hour

// ClientはRiverクライアントと、それが所有するデータベース接続をまとめて保持し、
// 起動と停止を1つの単位として扱えるようにする。
type Client struct {
	riverClient *river.Client[*sql.Tx]
	db          *database.DB
}

// NewClientはデータベースファイルへの専用の接続を開き、その上にRiver
// クライアントを構築する。書き込み用コネクションをリクエスト処理用と分けるのは、Riverが
// 自身のスケジュールで (空きジョブの探索やリーダー選出のために) 書き込むためで、単一の
// ライターを共有するとアプリケーションの書き込みがその後ろに並んでしまう。ワーカー専用の
// 依存は (注入ではなく) 本関数内でcfgから構築し、ここに閉じ込めてDIグラフの他の部分へ
// 漏らさない。
func NewClient(ctx context.Context, databasePath string, cfg *config.Config) (*Client, error) {
	// ワーカー専用のメール依存をcfgから構築し、メールワーカー (メール確認・
	// パスワードリセット・メールアドレス変更通知) を登録する。senderをデータベース接続を
	// 開く前に構築するのは、メール設定が拒否された場合に閉じるべき接続を残さず失敗させる
	// ため。ここでsenderを構築することで、リクエスト経路のコードが必要としない
	// プロバイダー設定をmain.goのDIグラフから締め出す。メール種別ごとの各senderは
	// 1つの基盤senderを共有する。
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

	// 退会済みユーザーのパージUseCaseをワーカー自身の接続上に構築する。メールジョブと
	// 違いDBアクセスが必要なため、リポジトリをここでその接続から構築する。これは
	// ワーカー専用の依存 (定期パージジョブだけが使う) のため、main.goのリクエスト経路の
	// DIグラフには載せない。
	purgeUserRepo := repository.NewUserRepository(db)
	purgeWithdrawnUsersUC := usecase.NewPurgeWithdrawnUsersUsecase(purgeUserRepo)

	workers := river.NewWorkers()
	river.AddWorker(workers, NewSendEmailConfirmationWorker(sendEmailConfirmationUC))
	river.AddWorker(workers, NewSendPasswordResetWorker(sendPasswordResetUC))
	river.AddWorker(workers, NewSendEmailChangeNotificationWorker(sendEmailChangeNotificationUC))
	river.AddWorker(workers, NewPurgeWithdrawnUsersWorker(purgeWithdrawnUsersUC))

	// パージジョブを日次の定期ジョブとして登録する。コンストラクタはArgsを自身の
	// InsertOptsと一緒に返すため、MaxAttemptsの既定値が適用される (nilのoptsを返すと
	// 失われる)。RunOnStartは付けない (定期ジョブのoptsはnil): 起動時に急いでパージする
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

	// Logger: slog.Default() によりRiver自身のジョブ実行・リトライログを構造化
	// ロガー経由で出力する。ワーカーのWorkメソッドが返すエラーもRiverが記録する。
	riverClient, err := river.NewClient(riversqlite.New(db.Writer), &river.Config{
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}},
		Workers:      workers,
		Logger:       slog.Default(),
		PeriodicJobs: periodicJobs,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ワーカーのRiverクライアントの作成に失敗: %w", err)
	}

	return &Client{
		riverClient: riverClient,
		db:          db,
	}, nil
}

// Startはジョブの取得と処理を開始する。
func (c *Client) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Riverクライアントを起動します")
	return c.riverClient.Start(ctx)
}

// Stopは実行中のジョブをドレインしてクライアントを停止し、所有するデータベース
// 接続を閉じる。
func (c *Client) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Riverクライアントを停止します")
	if err := c.riverClient.Stop(ctx); err != nil {
		return err
	}
	return c.db.Close()
}

// Clientは基盤のRiverクライアントを公開し、ジョブ投入のためDispatcherに
// 配線できるようにする (dispatcher.JobInserterを満たす)。
func (c *Client) Client() *river.Client[*sql.Tx] {
	return c.riverClient
}

// newEmailSenderはメール種別ごとのSenderが共有する基盤Senderを構築し、設定が
// 指定するtransportを選ぶ。プロバイダー未設定はResendに解決する。これにより、設定を
// 持たずに構築されたConfig (テストや、この設定が存在する前のデプロイ) は従来のtransportを
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
