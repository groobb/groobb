// Package seed rebuilds a development database with the data the screens cannot
// be looked at without: the accounts that sign in, and the community content
// they hold.
//
// A run empties every table it manages before generating, so that what is on
// screen is always what the current code produces. The rows go through the
// repositories the application itself writes with wherever it already has a
// Create; seeding is not a reason to grow the Infrastructure layer with code
// only the seed calls.
//
// A run refuses to start outside a development environment. It destroys the rows
// it manages and creates accounts whose password a file on disk chooses, so
// against anything else both have to be impossible rather than merely
// discouraged.
//
// [Ja] seed パッケージは、画面を見るために欠かせないデータ (サインインするアカウントと、
// それらが持つコミュニティの中身) で開発用データベースを作り直します。
//
// 実行は生成の前に管理対象のテーブルをすべて空にします。画面に出るデータが常に現在の
// コードの生成結果と一致するようにするためです。行の書き込みには、既に Create がある
// 対象ではアプリケーション自身が書き込みに使うリポジトリを通します。シードのために、
// シードだけが呼ぶコードを Infrastructure 層へ増やすことはしません。
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

// devEnv is the only environment a run is allowed in. It is compared as a raw
// string rather than through config.Config.IsDev so that a command can apply the
// same check to APP_ENV before config.Load substitutes its development default
// for an unset value: an unconfigured production process would otherwise pass a
// check made after loading.
//
// [Ja] devEnv は実行を許可する唯一の環境です。config.Config.IsDev ではなく生の文字列と
// 比較するのは、config.Load が未設定時の既定値を補う前の APP_ENV に対して、コマンド側が
// 同じ検査を適用できるようにするためです。そうしないと、設定されていない本番プロセスが、
// 読み込みの後に行う検査を通過してしまいます。
const devEnv = "dev"

// envFileKey is the configuration file key that carries the environment for
// everything except the guard here. The refusal names it so that it says which
// input it did not read, rather than leaving an operator who wrote it in the
// file to conclude that the file was ignored altogether.
//
// [Ja] envFileKey は、ここのガード以外のすべてに対して環境を与える設定ファイルのキー
// です。拒否の文言がこれを名指しすることで、どの入力を読まなかったのかを述べられます。
// そうしなければ、ファイルにこれを書いた運用者が、ファイルごと無視されたと結論することに
// なります。
const envFileKey = "app.env"

// Runner performs one seeding run against db.
//
// [Ja] Runner は db に対するシード実行 1 回分を受け持ちます。
type Runner struct {
	db         *database.DB
	cfg        *config.Config
	out        io.Writer
	rosterPath string
	profile    Profile
}

// NewRunner returns a Runner that writes its progress to out and generates the
// community profile describes.
//
// [Ja] NewRunner は、進捗を out に書き、profile が述べるコミュニティを生成する Runner を
// 返します。
func NewRunner(db *database.DB, cfg *config.Config, out io.Writer, profile Profile) *Runner {
	return &Runner{db: db, cfg: cfg, out: out, rosterPath: rosterPath, profile: profile}
}

// state carries what each generator produced to the generators that follow it,
// along with what the run read before the first one.
//
// [Ja] state は各生成器が作ったものを、後続の生成器へ引き渡します。実行が最初の生成器の
// 前に読んだものも合わせて運びます。
type state struct {
	roster *userRoster
	users  *seededUsers
	boards []seededBoard
}

// generator is one named step of a run. Holding the steps as a list keeps the
// order in one place and lets the phase numbers be counted off at run time,
// instead of being written into comments that drift from what actually runs.
//
// [Ja] generator は実行 1 回分の名前付きステップです。ステップを一覧として持つことで
// 順序が 1 箇所にまとまり、フェーズ番号を実行時に採番できます。番号をコメントへ書き込むと、
// 実際に走る内容とずれていくためです。
type generator struct {
	name string
	run  func(ctx context.Context, tx *sql.Tx, st *state) error
}

// Run empties the database and generates the seed data.
//
// Everything a run writes, the emptying included, goes through one transaction.
// A run that fails partway therefore leaves the database as it was, rather than
// leaving the developer with a database that has been emptied and not refilled.
//
// [Ja] Run はデータベースを空にしてシードデータを生成します。
//
// 空にする処理を含め、実行が書き込むものはすべて 1 つのトランザクションを通ります。
// そのため途中で失敗した実行はデータベースを元のまま残し、空にされたきり埋め直されて
// いないデータベースを開発者に残すことがありません。
func (r *Runner) Run(ctx context.Context) error {
	if err := EnsureDevEnv(r.cfg.Env); err != nil {
		return err
	}

	// Read the roster before touching the database: a roster that cannot be read
	// is a mistake in a file, and it should surface before anything has been
	// deleted.
	//
	// [Ja] データベースへ触れる前に名簿を読みます。名簿を読めないのはファイルの誤りで
	// あり、何かを削除する前に表面化させたいためです。
	roster, err := loadUserRoster(r.rosterPath)
	if err != nil {
		return err
	}

	// Report which database is about to be emptied, and which roster the accounts
	// come from. The command destroys every row it manages, so the developer gets
	// to see the target before the deletion rather than after it. The roster is
	// named beside it because which accounts a run creates depends on a file that
	// is not in version control.
	//
	// [Ja] これから空にするデータベースと、アカウントの供給元となる名簿を報告します。
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

// reportAccounts closes a run by naming the accounts it created, each under the
// role a generator asks for it by and beside the note that says what it is there
// to look at. An active account's line gives the address to sign in with for the
// screen being checked. The withdrawn role is also listed to identify the seed
// account behind authorless content, but its line records the original address
// as history rather than a usable sign-in credential.
//
// The accounts are named only when a generator actually produced them: state's
// fields are nil until the step that fills them has run, and the list of
// generators may be extended, so the closing report must not assume that any
// particular step took place.
//
// [Ja] reportAccounts は、実行が作成したアカウントを名指しして実行を締めます。各
// アカウントは、生成器がそれを求めるときの役割の名前で、そのアカウントが何を見るために
// いるのかを述べる覚え書きと並べて出します。有効なアカウントの行からは、確認したい画面に
// サインインするためのアドレスを読み取れます。退会済みロールも作者が表示されないコンテンツの
// 生成元を識別するために列挙しますが、その行は元のアドレスを履歴として記録するものであり、
// サインインには使えません。
//
// アカウントは、生成器が実際に作った場合にだけ出力します。state のフィールドはそれを埋める
// ステップが走るまで nil であり、生成器の一覧は今後追加されるため、締めの報告が特定の
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

// EnsureDevEnv rejects any environment other than development.
//
// It takes the environment name rather than a *config.Config so that a command
// can apply the check to the raw APP_ENV, before config.Load substitutes its
// development default for an unset value. Every guard calls this one function,
// so the wording of the refusal cannot drift apart, and the refusal names what it
// covers rather than the seed alone.
//
// The refusal says that the configuration file is not consulted. Every other
// setting resolves from either input, so an operator who wrote env in the file
// would otherwise read `APP_ENV is ""` as a claim that they had configured
// nothing.
//
// [Ja] EnsureDevEnv は開発環境以外での実行を拒否します。
//
// *config.Config ではなく環境名を受け取るのは、config.Load が未設定時の既定値を補う前の
// 生の APP_ENV に対しても検査を適用できるようにするためです。すべてのガードがこの 1 つの
// 関数を呼ぶため、拒否の文言がずれることがなく、文言もシードだけでなく対象全体を名指し
// する形にしています。
//
// 拒否の文言は、設定ファイルを参照しないことを述べます。他の設定はどちらの入力からも
// 解決されるため、そう書かなければ、ファイルに env を書いた運用者が `APP_ENV is ""` を
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
