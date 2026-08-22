// Package config provides loading and access to application settings, which
// come from a TOML configuration file and from environment variables.
//
// [Ja] config パッケージは、TOML の設定ファイルと環境変数から成るアプリケーション設定を
// 読み込み、アクセスする機能を提供します。
package config

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"time"
)

// ContinuationTokenMinimumKeyLength is the shortest ContinuationTokenKey that is
// accepted, in bytes. A length of 32 bytes matches SHA-256's 256-bit output.
// The configured value must still come from a cryptographically secure random
// source: this length check rejects short keys but cannot establish their
// entropy.
//
// It lives here rather than beside the signing code because Load rejects a short
// key at startup while the signing code fails closed on one, and both must draw
// the bound from the same place: raising it in only one of the two would leave a
// key that starts the application but signs nothing.
//
// [Ja] ContinuationTokenMinimumKeyLength は受け付ける ContinuationTokenKey の最小長
// (バイト) です。32 バイトは SHA-256 の 256 bit の出力長に対応します。設定値自体は
// 暗号学的に安全な乱数源から生成する必要があり、この長さ検査は短い鍵を拒否しますが、
// エントロピーまでは保証できません。
//
// 署名処理の隣ではなくここに置くのは、Load が起動時に短い鍵を拒否し、署名処理側は短い鍵で
// fail-closed になるという 2 つの判定が同じ値を見る必要があるためです。片方だけを引き上げ
// ると、アプリケーションは起動するのに何も署名できない鍵が生まれます。
const ContinuationTokenMinimumKeyLength = 32

// Config holds the application settings.
//
// [Ja] Config はアプリケーションの設定を保持します。
type Config struct {
	// Env is the running environment: "dev", "test", or "prod".
	//
	// [Ja] Env は実行環境 ("dev" / "test" / "prod") を表します。
	Env string

	// ContinuationTokenKey signs the short-lived cookies that carry server-side
	// state between steps of the email-confirmation and two-factor sign-in flows.
	// It must be a stable, secret value of at least 32 bytes: changing it
	// invalidates outstanding continuation tokens, while exposing it lets an
	// attacker forge authentication state.
	//
	// [Ja] ContinuationTokenKey はメール確認と 2 段階認証サインインの各ステップ間で
	// サーバー側状態を運ぶ短命 Cookie に署名します。32 バイト以上の安定した秘密値で
	// なければなりません。変更すると発行済み continuation token が無効になり、漏えいすると
	// 攻撃者が認証状態を偽造できるためです。
	ContinuationTokenKey string

	// DatabasePath is the filesystem path of the SQLite database file. It holds
	// every piece of state an instance keeps, so it is required rather than
	// defaulted: a plausible-looking default would let a misconfigured instance
	// start on an empty database of its own making instead of reporting that it
	// does not know where its data is.
	//
	// [Ja] DatabasePath は SQLite データベースファイルのファイルシステム上のパスです。
	// インスタンスが保持する状態はすべてこのファイルにあるため、既定値を持たせず必須と
	// します。それらしい既定値を置くと、設定を誤ったインスタンスが「データの在り処が
	// 分からない」と報告する代わりに、自分で作った空のデータベースの上で起動して
	// しまうためです。
	DatabasePath string

	// Port is the TCP port the HTTP server listens on.
	//
	// [Ja] Port は HTTP サーバーが待ち受ける TCP ポートです。
	Port string

	// AssetVersion is the cache-busting value used for static assets in non-dev
	// environments. It is fixed at startup from the value stamped into the binary
	// at build time (see buildAssetVersion). In dev a timestamp is used instead;
	// see GetAssetVersion.
	//
	// [Ja] AssetVersion は非開発環境で静的アセットに使うキャッシュ無効化用の値です。
	// ビルド時にバイナリへ埋め込まれた値から起動時に固定します (buildAssetVersion を
	// 参照)。開発環境では代わりにタイムスタンプを使います (GetAssetVersion を参照)。
	AssetVersion string

	// ResendAPIKey, EmailFrom, and EmailFromName configure outgoing email. The
	// worker client consumes them when it builds the email sender for background
	// jobs; the API key is used only when EmailProvider selects Resend, while the
	// From address and name are used by either provider. They are optional rather
	// than required so that an instance whose mail is not provisioned yet still
	// boots and can be reached to finish configuring itself; a send then fails on
	// the job instead of at startup.
	//
	// [Ja] ResendAPIKey / EmailFrom / EmailFromName は送信メールを設定します。ワーカー
	// クライアントがバックグラウンドジョブ用の email sender を構築する際に使い、API キーは
	// EmailProvider が Resend を選んだときにのみ、From アドレスと名前はどちらの
	// プロバイダーでも使います。必須ではなく任意とするのは、メールが未設定のインスタンスでも
	// 起動して設定を仕上げるために到達できる必要があるためです。この場合、送信は起動時では
	// なくジョブ側で失敗します。
	ResendAPIKey string

	// EmailFrom is the sender address used in the From header of outgoing email.
	//
	// [Ja] EmailFrom は送信メールの From ヘッダーに使う送信元アドレスです。
	EmailFrom string

	// EmailFromName is the display name shown alongside EmailFrom in the From
	// header.
	//
	// [Ja] EmailFromName は From ヘッダーで EmailFrom と並べて表示する送信元の表示名
	// です。
	EmailFromName string

	// AppURL is the public base URL of the application (e.g.
	// "https://groobb.example.dev" in production, "http://localhost:8080" in
	// dev), with no trailing slash. It is needed to build absolute links in
	// outgoing email (such as the password reset link), which a relative path
	// cannot express. A full base URL is stored rather than a bare domain so it
	// can carry the scheme and port that dev (plain HTTP on a port) requires. It
	// is optional rather than required for the same reason as the email settings
	// (a deployment without email configured must still boot); when the link is
	// built from an empty AppURL the URL is simply host-relative.
	//
	// [Ja] AppURL はアプリケーションの公開ベース URL (例: 本番は
	// "https://groobb.example.dev"、dev は "http://localhost:8080") で、末尾スラッシュは
	// 付けません。送信メール内の絶対リンク (パスワードリセットリンクなど) を組み立てるのに
	// 必要で、相対パスでは表現できません。素のドメインではなくベース URL 全体を保持するのは、
	// dev (ポート上の平文 HTTP) が要求するスキームとポートを含められるようにするためです。
	// メール設定と同じ理由で必須ではなく任意とします (メール未設定のデプロイでも起動できる
	// 必要があるため)。空の AppURL からリンクを組み立てた場合、URL は単にホスト相対になります。
	AppURL string

	// TurnstileSiteKey and TurnstileSecretKey configure Cloudflare Turnstile bot
	// protection on the public forms. The site key is handed to templates to
	// render the widget, and the secret key is used server-side to verify the
	// submitted token. They are optional: when both are empty (a disabled dev /
	// test setup), the widget is not rendered and token verification is bypassed.
	//
	// [Ja] TurnstileSiteKey と TurnstileSecretKey は公開フォームの Cloudflare
	// Turnstile による Bot 対策を設定します。サイトキーはウィジェットを描画するために
	// テンプレートへ渡し、シークレットキーは送信されたトークンをサーバー側で検証するのに
	// 使います。任意であり、両方が空のとき (dev / test の無効化時) はウィジェットを描画せず、
	// トークン検証もバイパスします。
	TurnstileSiteKey   string
	TurnstileSecretKey string
}

// Load reads the configuration from the TOML configuration file and from
// environment variables, with the environment taking precedence per setting.
//
// The file is optional: without one, every setting comes from the environment,
// which is how development (`op run --env-file=.env` resolves the values) and
// CI run. A self-hosted instance is expected to keep its settings in the file
// and reach for the environment to override one of them, or to hold the secrets
// its deployment injects.
//
// [Ja] Load は TOML の設定ファイルと環境変数から設定を読み込みます。設定ごとに、
// 環境変数がファイルより優先されます。
//
// 設定ファイルは任意です。ファイルが無ければすべての設定は環境変数から読み込まれ、
// 開発環境 (`op run --env-file=.env` が値を解決する) と CI はこの形で動きます。
// セルフホストのインスタンスは設定をファイルに置き、環境変数は個別の上書きや、
// デプロイ側が注入する秘密情報のために使うことを想定します。
func Load() (*Config, error) {
	file, err := loadFile()
	if err != nil {
		return nil, err
	}

	// The running environment defaults to "dev" when neither source sets it.
	//
	// [Ja] 実行環境はどちらの入力も設定していない場合 "dev" を既定値とします。
	env := newSetting("APP_ENV", "app.env", file.App.Env)
	cfg := &Config{Env: env.value}
	if cfg.Env == "" {
		cfg.Env = "dev"
	}

	port := newSetting("GROOBB_PORT", "server.port", intFileValue(file.Server.Port))
	if !port.isSet() {
		return nil, port.missingError()
	}
	if _, err := port.tcpPort("server port"); err != nil {
		return nil, err
	}
	cfg.Port = port.value

	databasePath := newSetting("GROOBB_DATABASE_PATH", "database.path", file.Database.Path)
	if !databasePath.isSet() {
		return nil, databasePath.missingError()
	}
	cfg.DatabasePath = databasePath.value

	continuationTokenKey := newSetting("GROOBB_CONTINUATION_TOKEN_KEY", "security.continuation_token_key", file.Security.ContinuationTokenKey)
	if !continuationTokenKey.isSet() {
		return nil, continuationTokenKey.missingError()
	}
	if len(continuationTokenKey.value) < ContinuationTokenMinimumKeyLength {
		return nil, fmt.Errorf("the continuation token key from %s must be at least %d bytes", continuationTokenKey.source(), ContinuationTokenMinimumKeyLength)
	}
	cfg.ContinuationTokenKey = continuationTokenKey.value

	// Fix the asset version once at startup so that non-dev environments serve
	// stable, cache-busting asset URLs for the lifetime of a deploy.
	//
	// [Ja] 非開発環境がデプロイの間ずっと安定したキャッシュ無効化 URL を配信できるよう、
	// アセットバージョンを起動時に一度だけ固定します。
	cfg.AssetVersion = buildAssetVersion(assetVersion, vcsRevision())

	// Email settings are read without requiring them: an instance whose mail is
	// not provisioned yet must still boot so that it can be reached to finish
	// configuring itself (see the field docs).
	//
	// [Ja] メール設定は必須にせず読み込む。メールが未設定のインスタンスでも、設定を
	// 仕上げるために到達できるよう起動できる必要がある (フィールドのドキュメントを
	// 参照)。
	cfg.ResendAPIKey = newSetting("GROOBB_RESEND_API_KEY", "email.resend_api_key", file.Email.ResendAPIKey).value
	cfg.EmailFrom = newSetting("GROOBB_EMAIL_FROM", "email.from", file.Email.From).value
	cfg.EmailFromName = newSetting("GROOBB_EMAIL_FROM_NAME", "email.from_name", file.Email.FromName).value

	// AppURL is read without requiring it, for the same reason as the email
	// settings above (see the field docs).
	//
	// [Ja] AppURL は必須にせず読み込む。理由は上のメール設定と同じ (フィールドの
	// ドキュメントを参照)。
	cfg.AppURL = newSetting("GROOBB_APP_URL", "app.url", file.App.URL).value

	// Turnstile keys are read without requiring them: Turnstile is enabled
	// operationally by provisioning the real keys, so a deployment must still boot
	// before they are set (see the field docs).
	//
	// [Ja] Turnstile キーは必須にせず読み込む。Turnstile は実キーを設定する運用手順で
	// 有効化するため、キー設定前でもデプロイが起動できる必要がある (フィールドの
	// ドキュメントを参照)。
	cfg.TurnstileSiteKey = newSetting("GROOBB_TURNSTILE_SITE_KEY", "turnstile.site_key", file.Turnstile.SiteKey).value
	cfg.TurnstileSecretKey = newSetting("GROOBB_TURNSTILE_SECRET_KEY", "turnstile.secret_key", file.Turnstile.SecretKey).value

	// The disable flag lets non-production environments switch Turnstile off with
	// a single setting instead of unsetting both keys. When enabled, both keys are
	// cleared so the empty-key path takes over: token verification is bypassed and
	// the widget is not rendered.
	//
	// In production the flag is deliberately ignored (fail-closed) and only logged
	// as a warning. Turnstile is bot protection, so a stray disable flag leaking
	// into production must never silently disable it.
	//
	// [Ja] 無効化フラグは、非本番環境で 2 つのキーを未設定にする代わりに 1 つの設定で
	// Turnstile を無効化するためのものです。有効時は両キーを空に落とし、キー空の経路
	// (トークン検証がバイパスされ、ウィジェットも描画されない) に委ねます。
	//
	// 本番ではフラグを意図的に無視し (fail-closed)、warn ログを出すだけにします。
	// Turnstile は Bot 対策なので、無効化フラグが誤って本番に漏れても、黙って無効化されては
	// なりません。
	if newSetting("GROOBB_TURNSTILE_DISABLE", "turnstile.disable", boolFileValue(file.Turnstile.Disable)).value == "true" {
		if cfg.IsProduction() {
			slog.Warn("Turnstile の無効化設定は本番環境では無視されます (Bot 対策を維持するため fail-closed)")
		} else {
			cfg.TurnstileSiteKey = ""
			cfg.TurnstileSecretKey = ""
		}
	}

	// In production, warn when exactly one of the two keys is set. A site key
	// without a secret key still renders the widget while server-side
	// verification is bypassed (empty secret), so bot protection would be
	// silently off despite looking active. This is kept a warning rather than a
	// startup error because the keys are optional (a deployment must still boot
	// before they are provisioned); both keys empty is the deliberate
	// "not enabled yet" state and is not warned.
	//
	// [Ja] 本番では 2 つのキーのうち片方だけが設定されているときに警告します。
	// シークレットキーのないサイトキーは、ウィジェットを描画しつつサーバー側の検証を
	// バイパスする (シークレット空) ため、有効に見えて Bot 対策が黙って無効になります。
	// キーは任意 (設定前でもデプロイが起動できる必要がある) なので起動時エラーにはせず
	// 警告に留めます。両キーが空の状態は「まだ導入していない」意図的な状態として警告しません。
	if cfg.IsProduction() && (cfg.TurnstileSiteKey == "") != (cfg.TurnstileSecretKey == "") {
		slog.Warn("Turnstile のキーが片方のみ設定されています (本番で Bot 対策が黙って無効化される恐れ)")
	}

	return cfg, nil
}

// LogValue renders the configuration for structured logging with its secrets
// replaced, so that logging a Config cannot put the values that authenticate
// the instance into the log. Whether a secret is set is kept, which is what a
// question about the configuration needs to distinguish.
//
// It exists because slog renders a struct field by field: without it, a single
// call that logs a Config anywhere would write out the continuation token key,
// the Resend API key, and the Turnstile secret key.
//
// [Ja] LogValue は構造化ログ向けに設定を整形し、秘密情報を置き換えます。Config を
// ログに出しても、インスタンスの認証に使う値がログへ入らないようにするためです。
// 秘密情報が設定されているかどうかは残します。設定についての疑問を切り分けるのに
// 必要なのはそこだからです。
//
// これが必要なのは、slog が構造体をフィールドごとに展開するためです。これが無いと、
// どこか 1 箇所で Config をログに渡すだけで、continuation token の鍵・Resend の
// API キー・Turnstile のシークレットキーが書き出されます。
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", c.Env),
		slog.String("port", c.Port),
		slog.String("database_path", c.DatabasePath),
		slog.String("asset_version", c.AssetVersion),
		slog.String("app_url", c.AppURL),
		slog.String("email_from", c.EmailFrom),
		slog.String("email_from_name", c.EmailFromName),
		slog.String("turnstile_site_key", c.TurnstileSiteKey),
		slog.String("continuation_token_key", redactSecret(c.ContinuationTokenKey)),
		slog.String("resend_api_key", redactSecret(c.ResendAPIKey)),
		slog.String("turnstile_secret_key", redactSecret(c.TurnstileSecretKey)),
	)
}

// redactedSecret is what LogValue shows in place of a secret that is set.
//
// [Ja] redactedSecret は、設定済みの秘密情報の代わりに LogValue が表示する値です。
const redactedSecret = "[REDACTED]"

// redactSecret replaces a secret with a marker, keeping an unset one empty so
// that the log still shows which secrets an instance is missing.
//
// [Ja] redactSecret は秘密情報をマーカーに置き換えます。未設定のものは空のままにし、
// どの秘密情報が欠けているかはログから分かるようにします。
func redactSecret(secret string) string {
	if secret == "" {
		return ""
	}

	return redactedSecret
}

// IsDev reports whether the running environment is development.
//
// [Ja] IsDev は実行環境が開発環境かどうかを返します。
func (c *Config) IsDev() bool {
	return c.Env == "dev"
}

// IsTest reports whether the running environment is test.
//
// [Ja] IsTest は実行環境がテスト環境かどうかを返します。
func (c *Config) IsTest() bool {
	return c.Env == "test"
}

// IsProduction reports whether the running environment is production.
//
// [Ja] IsProduction は実行環境が本番環境かどうかを返します。
func (c *Config) IsProduction() bool {
	return c.Env == "prod"
}

// GetAssetVersion returns the cache-busting value appended to static asset URLs.
//
// In dev it returns a fresh millisecond timestamp on every call so that edits
// to CSS / JS are picked up without manual cache clearing. In other
// environments it returns the static AssetVersion fixed at startup.
//
// [Ja] GetAssetVersion は静的アセットの URL に付与するキャッシュ無効化用の値を
// 返します。
//
// 開発環境では呼び出しごとに新しいミリ秒タイムスタンプを返し、CSS / JS の編集を
// 手動のキャッシュクリアなしに反映できるようにします。それ以外の環境では、起動時に
// 固定した静的な AssetVersion を返します。
func (c *Config) GetAssetVersion() string {
	if c.IsDev() {
		return strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	return c.AssetVersion
}

// assetVersion is stamped into the binary at build time with
// `-ldflags "-X github.com/groobb/groobb/go/internal/config.assetVersion=..."`
// (see the build target in go/Makefile). It is a package-level variable because
// that is the only thing the linker can write to, and it is left empty so that a
// build without the flag falls back to the revision below.
//
// [Ja] assetVersion はビルド時に
// `-ldflags "-X github.com/groobb/groobb/go/internal/config.assetVersion=..."`
// でバイナリへ埋め込まれます (go/Makefile の build ターゲットを参照)。リンカが書き込め
// るのはパッケージレベルの変数だけのためこの形とし、フラグ無しのビルドが下のリビジョンへ
// フォールバックできるよう空のままにしています。
var assetVersion string

// buildAssetVersion picks the asset version from the value stamped in at build
// time, falling back to the revision the binary was built from and finally to
// "dev".
//
// The fallbacks matter because a value that never changes serves stale CSS: an
// operator who builds from source with a plain `go build` gets no stamp, and
// pinning every such build to one constant would leave an upgraded instance
// serving the assets the browser already cached.
//
// [Ja] buildAssetVersion は、ビルド時に埋め込まれた値からアセットバージョンを決め、
// 無ければバイナリのビルド元リビジョン、最後に "dev" へフォールバックします。
//
// フォールバックが重要なのは、変化しない値が古い CSS を配信するためです。ソースから素の
// `go build` でビルドする運用者には値が埋め込まれず、そうしたビルドをすべて 1 つの定数に
// 固定すると、更新したインスタンスがブラウザのキャッシュ済みアセットを使わせ続けます。
func buildAssetVersion(stamped, revision string) string {
	if stamped != "" {
		return stamped
	}
	if revision != "" {
		return revision
	}
	return "dev"
}

// vcsRevision returns the complete revision the binary was built from, or an
// empty string when the build carries no version control information (a build
// from a source archive, or a test binary). Keeping the complete value matches
// the linker stamp and prevents abbreviated-prefix collisions from reusing a
// long-lived asset cache key.
//
// It reads the build information the toolchain records rather than running
// `git`, because a distributed binary has neither a repository nor a git
// executable around it at run time.
//
// [Ja] vcsRevision はバイナリのビルド元となった完全なリビジョンを返します。ビルドが
// バージョン管理の情報を持たない場合 (ソースアーカイブからのビルドやテストバイナリ) は
// 空文字列を返します。完全な値を保つことで linker stamp と揃え、短縮した接頭辞の衝突に
// よって長期のアセットキャッシュキーが再利用されないようにします。
//
// `git` を実行せずツールチェインが記録したビルド情報を読むのは、配布されたバイナリの
// 周囲には実行時にリポジトリも git の実行ファイルも無いためです。
func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	return vcsRevisionFromSettings(info.Settings)
}

// vcsRevisionFromSettings returns the complete revision from build settings, or
// an empty string when the revision setting is absent. Keeping the extraction
// separate from debug.ReadBuildInfo makes every branch deterministic to test.
//
// [Ja] vcsRevisionFromSettings はビルド設定から完全なリビジョンを返し、リビジョン設定が
// 無い場合は空文字列を返します。抽出を debug.ReadBuildInfo から分けることで、すべての
// 分岐を決定的にテストできるようにします。
func vcsRevisionFromSettings(settings []debug.BuildSetting) string {
	for _, setting := range settings {
		if setting.Key != "vcs.revision" {
			continue
		}
		return setting.Value
	}

	return ""
}
