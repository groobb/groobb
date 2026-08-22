package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// configFileEnvName names the environment variable that points at the
	// configuration file. A path given there is the operator's deliberate
	// choice, so a file missing from it stops startup instead of falling back to
	// the environment alone: otherwise a typo in the path would start an instance
	// that silently ignores every setting the file holds.
	//
	// [Ja] configFileEnvName は設定ファイルの場所を指す環境変数の名前です。ここで渡された
	// パスは運用者が明示的に選んだものなので、そこにファイルが無い場合は環境変数だけに
	// フォールバックせず起動を止めます。そうしないと、パスの打ち間違いが、ファイルの設定を
	// すべて黙って無視するインスタンスを起動させてしまいます。
	configFileEnvName = "GROOBB_CONFIG_FILE"

	// defaultConfigFileName is the file Load reads from the working directory
	// when configFileEnvName is unset. Its absence is not an error, because an
	// instance configured entirely through the environment (a container, CI, and
	// the development setup here) must still start.
	//
	// [Ja] defaultConfigFileName は configFileEnvName が未設定のときに Load が作業
	// ディレクトリから読むファイルです。存在しないことはエラーにしません。環境変数だけで
	// 設定するインスタンス (コンテナ・CI・ここの開発環境) も起動できる必要があるためです。
	defaultConfigFileName = "groobb.toml"
)

// fileConfig is the schema of the configuration file: one field per setting,
// grouped into the tables an operator edits.
//
// The tables follow the environment variable names, with the component a
// variable names becoming the table (GROOBB_SMTP_HOST is "host" under
// [email.smtp], APP_ENV is "env" under [app]), so that each setting has one
// name on each side and the two can be read off each other. No setting sits at
// the top level: in TOML a key written after a table belongs to that table, so
// a top-level key would be a trap in a hand-edited file.
//
// A numeric setting is a pointer so that a written value is distinguishable
// from an absent key. Both ports here reject 0, and folding it into "absent"
// would answer an operator who wrote `port = 0` with a message saying the
// setting is not configured. A list setting needs no such distinction: an
// empty list and an absent key both leave the setting off.
//
// [Ja] fileConfig は設定ファイルのスキーマです。設定 1 つにつき 1 フィールドを持ち、
// 運用者が編集する単位でテーブルにまとめています。
//
// テーブルの分け方は環境変数の名前に従い、変数名が示す構成要素をテーブルにします
// (GROOBB_SMTP_HOST は [email.smtp] の "host"、APP_ENV は [app] の "env")。
// これにより設定ごとの名前が両者で 1 対 1 に対応し、一方から他方を読み取れます。
// トップレベルに置く設定はありません。TOML ではテーブルより後ろに書いたキーはその
// テーブルに属するため、トップレベルのキーは手で編集するファイルでは罠になります。
//
// 数値の設定はポインタにして、書かれた値とキーの不在を区別できるようにしています。
// ここにある 2 つのポートはどちらも 0 を拒否しますが、0 を「不在」に畳み込むと、
// `port = 0` と書いた運用者に「設定されていない」と返してしまうためです。リストの設定に
// この区別は要りません。空のリストとキーの不在は、どちらも設定を無効のままにします。
type fileConfig struct {
	App       fileApp       `toml:"app"`
	Server    fileServer    `toml:"server"`
	Database  fileDatabase  `toml:"database"`
	Security  fileSecurity  `toml:"security"`
	Email     fileEmail     `toml:"email"`
	Turnstile fileTurnstile `toml:"turnstile"`
}

type fileApp struct {
	Env string `toml:"env"`
	URL string `toml:"url"`
}

type fileServer struct {
	Port           *int     `toml:"port"`
	TrustedProxies []string `toml:"trusted_proxies"`
}

type fileDatabase struct {
	Path string `toml:"path"`
}

type fileSecurity struct {
	ContinuationTokenKey string `toml:"continuation_token_key"`
}

type fileEmail struct {
	Provider     string   `toml:"provider"`
	From         string   `toml:"from"`
	FromName     string   `toml:"from_name"`
	ResendAPIKey string   `toml:"resend_api_key"`
	SMTP         fileSMTP `toml:"smtp"`
}

type fileSMTP struct {
	Host     string `toml:"host"`
	Port     *int   `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	TLSMode  string `toml:"tls_mode"`
}

type fileTurnstile struct {
	SiteKey   string `toml:"site_key"`
	SecretKey string `toml:"secret_key"`
	Disable   bool   `toml:"disable"`
}

// loadFile reads the configuration file. It returns an empty fileConfig when
// the default file is not there, which leaves every setting to the environment.
//
// A key that matches no field is rejected rather than ignored: an operator who
// misspells a setting must not get an instance that starts as if the setting
// were not written at all.
//
// [Ja] loadFile は設定ファイルを読み込みます。既定のファイルが存在しない場合は空の
// fileConfig を返し、すべての設定を環境変数に委ねます。
//
// どのフィールドにも一致しないキーは無視せず拒否します。設定名を打ち間違えた運用者が、
// その設定を書いていないのと同じ状態で起動したインスタンスを得ないようにするためです。
func loadFile() (*fileConfig, error) {
	path, explicit := configFilePath()

	file := &fileConfig{}
	meta, err := toml.DecodeFile(path, file)
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return &fileConfig{}, nil
		}
		return nil, configFileError(path, err)
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("the configuration file %s has settings that do not exist: %s", path, formatKeys(undecoded))
	}

	slog.Info("設定ファイルを読み込みました", "path", path)

	return file, nil
}

// configFilePath returns the configuration file to read and whether the
// operator named it themselves.
//
// [Ja] configFilePath は読み込む設定ファイルと、それを運用者自身が指定したものかどうかを
// 返します。
func configFilePath() (path string, explicit bool) {
	if configured := os.Getenv(configFileEnvName); configured != "" {
		return configured, true
	}

	return defaultConfigFileName, false
}

// configFileError converts a failure to read or decode the configuration file
// into an error that names where the problem is without repeating what the file
// says there.
//
// A syntax error's message quotes the token the parser stumbled on (`expected
// value but found "hunter" instead` for an unquoted password), and this error
// is logged at startup, so the message is dropped and only the position and the
// last key parsed are kept. Everything else is passed through: the remaining
// decoding errors report types rather than values.
//
// [Ja] configFileError は、設定ファイルの読み込み・デコードの失敗を、問題の位置は示しつつ
// そこにファイルが何と書いてあるかは繰り返さないエラーへ変換します。
//
// 構文エラーのメッセージはパーサーがつまずいたトークンを引用するため (引用符の無い
// パスワードに対する `expected value but found "hunter" instead` など)、起動時にログへ
// 出るこのエラーからはメッセージを落とし、位置と直前に解析したキーだけを残します。
// それ以外はそのまま通します。残るデコードエラーが報告するのは値ではなく型です。
func configFileError(path string, err error) error {
	var parseErr toml.ParseError
	if !errors.As(err, &parseErr) {
		return fmt.Errorf("failed to read the configuration file %s: %w", path, err)
	}

	where := fmt.Sprintf("line %d", parseErr.Position.Line)
	if parseErr.LastKey != "" {
		where = fmt.Sprintf("%s (last key %q)", where, parseErr.LastKey)
	}

	return fmt.Errorf("failed to parse the configuration file %s at %s; the parser's message is omitted because it can quote the file, which holds secrets", path, where)
}

// formatKeys renders the keys of a configuration file for an error message.
//
// [Ja] formatKeys は設定ファイルのキーをエラーメッセージ向けに整形します。
func formatKeys(keys []toml.Key) string {
	formatted := make([]string, 0, len(keys))
	for _, key := range keys {
		formatted = append(formatted, fmt.Sprintf("%q", key.String()))
	}

	return strings.Join(formatted, ", ")
}
