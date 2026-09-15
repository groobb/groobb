// seedパッケージは、画面を見るために欠かせないデータ (サインインするアカウントと、
// それらが持つコミュニティの中身) で開発用データベースを作り直します。
//
// 実行は生成の前に管理対象のテーブルをすべて空にします。画面に出るデータが常に現在の
// コードの生成結果と一致するようにするためです。行の書き込みには、既にCreateがある
// 対象ではアプリケーション自身が書き込みに使うリポジトリを通します。シードのために、
// シードだけが呼ぶコードをInfrastructure層へ増やすことはしません。
//
// 実行は開発環境以外では開始を拒否します。管理対象の行を破棄し、ディスク上のファイルが
// 決めたパスワードでサインインできるアカウントを作るため、それ以外に対しては、どちらも
// 推奨しないのではなく実行できないようにする必要があります。
package seed

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
)

// devEnvは実行を許可する唯一の環境です。config.Config.IsDevではなく生の文字列と
// 比較するのは、config.Loadが未設定時の既定値を補う前のAPP_ENVに対して、コマンド側が
// 同じ検査を適用できるようにするためです。そうしないと、設定されていない本番プロセスが、
// 読み込みの後に行う検査を通過してしまいます。
const devEnv = "dev"

// envFileKeyは、ここのガード以外のすべてに対して環境を与える設定ファイルのキー
// です。拒否の文言がこれを名指しすることで、どの入力を読まなかったのかを述べられます。
// そうしなければ、ファイルにこれを書いた運用者が、ファイルごと無視されたと結論することに
// なります。
const envFileKey = "app.env"

// Runnerはdbに対するシード実行1回分を受け持ちます。
type Runner struct {
	db         *database.DB
	cfg        *config.Config
	out        io.Writer
	rosterPath string
	profile    Profile
}

// NewRunnerは、進捗をoutに書き、profileが述べるコミュニティを生成するRunnerを
// 返します。
func NewRunner(db *database.DB, cfg *config.Config, out io.Writer, profile Profile) *Runner {
	return &Runner{db: db, cfg: cfg, out: out, rosterPath: rosterPath, profile: profile}
}

// stateは各生成器が作ったものを、後続の生成器へ引き渡します。実行が最初の生成器の
// 前に読んだものも合わせて運びます。
type state struct {
	roster *userRoster
	users  *seededUsers
	boards []seededBoard
}

// generatorは実行1回分の名前付きステップです。ステップを一覧として持つことで
// 順序が1箇所にまとまり、フェーズ番号を実行時に採番できます。番号をコメントへ書き込むと、
// 実際に走る内容とずれていくためです。
type generator struct {
	name string
	run  func(ctx context.Context, tx *sql.Tx, st *state) error
}

// Runはデータベースを空にしてシードデータを生成します。
//
// 空にする処理を含め、実行が書き込むものはすべて1つのトランザクションを通ります。
// そのため途中で失敗した実行はデータベースを元のまま残し、空にされたきり埋め直されて
// いないデータベースを開発者に残すことがありません。
func (r *Runner) Run(ctx context.Context) error {
	if err := EnsureDevEnv(r.cfg.Env); err != nil {
		return err
	}

	// データベースへ触れる前に名簿を読みます。名簿を読めないのはファイルの誤りで
	// あり、何かを削除する前に表面化させたいためです。
	roster, err := loadUserRoster(r.rosterPath)
	if err != nil {
		return err
	}

	// これから空にするデータベースと、アカウントの供給元となる名簿を報告します。
	// 本コマンドは管理対象の行をすべて破棄するため、削除後ではなく削除前に対象を目視
	// できるようにします。名簿を並べて出すのは、実行がどのアカウントを作るのかが、
	// バージョン管理に入っていないファイルに依存しているためです。
	slog.InfoContext(ctx, "seeding the development database", "database_path", r.cfg.DatabasePath, "roster_path", roster.path, "profile", r.profile.name)

	startedAt := time.Now()

	tx, err := r.db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin the transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := cleanup(ctx, tx); err != nil {
		return err
	}
	slog.InfoContext(ctx, "emptied the tables the seed manages", "table_count", len(cleanupTables))

	generators := []generator{
		{name: "community", run: r.generateCommunity},
		{name: "users", run: r.generateUsers},
		{name: "user roles", run: r.generateUserRoles},
		{name: "boards", run: r.generateBoards},
		{name: "threads", run: r.generateThreads},
		{name: "withdrawal", run: r.generateWithdrawal},
	}

	st := &state{roster: roster}
	for i, g := range generators {
		slog.InfoContext(ctx, "generating the seed data", "phase", i+1, "phase_count", len(generators), "generator", g.name)
		if err := g.run(ctx, tx, st); err != nil {
			return fmt.Errorf("failed to generate the %s: %w", g.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit the transaction: %w", err)
	}

	r.reportAccounts(ctx, roster, st, startedAt)

	return nil
}

// reportAccountsは、実行が作成したアカウントを名指しして実行を締めます。各
// アカウントは、生成器がそれを求めるときの役割の名前で、そのアカウントが何を見るために
// いるのかを述べる覚え書きと並べて出します。有効なアカウントの行からは、確認したい画面に
// サインインするためのアドレスを読み取れます。退会済みロールも作者が表示されないコンテンツの
// 生成元を識別するために列挙しますが、その行は元のアドレスを履歴として記録するものであり、
// サインインには使えません。
//
// アカウントは、生成器が実際に作った場合にだけ出力します。stateのフィールドはそれを埋める
// ステップが走るまでnilであり、生成器の一覧は今後追加されるため、締めの報告が特定の
// ステップの実行を前提にしてはならないためです。
func (r *Runner) reportAccounts(ctx context.Context, roster *userRoster, st *state, startedAt time.Time) {
	attrs := []any{"elapsed", time.Since(startedAt).Round(time.Millisecond)}
	if st.users != nil {
		for _, account := range roster.users {
			if user := st.users.user(account.role); user != nil {
				attrs = append(attrs, string(account.role), fmt.Sprintf("%s (%s)", user.Email, account.note))
			}
		}
	}

	slog.InfoContext(ctx, "the seed data is in place", attrs...)
}

// EnsureDevEnvは開発環境以外での実行を拒否します。
//
// *config.Configではなく環境名を受け取るのは、config.Loadが未設定時の既定値を補う前の
// 生のAPP_ENVに対しても検査を適用できるようにするためです。すべてのガードがこの1つの
// 関数を呼ぶため、拒否の文言がずれることがなく、文言もシードだけでなく対象全体を名指し
// する形にしています。
//
// 拒否の文言は、設定ファイルを参照しないことを述べます。他の設定はどちらの入力からも
// 解決されるため、そう書かなければ、ファイルにenvを書いた運用者が `APP_ENV is ""` を
// 「何も設定していない」という主張として読むことになります。
func EnsureDevEnv(env string) error {
	if env != devEnv {
		return fmt.Errorf(
			"a command that handles development data can only run in a development environment, but APP_ENV is %q; "+
				"this command reads APP_ENV alone, so %q in the configuration file does not enable it",
			env, envFileKey,
		)
	}

	return nil
}
