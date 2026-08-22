// Package config provides loading and access to application settings, which
// come from a TOML configuration file and from environment variables.
//
// [Ja] config パッケージは、TOML の設定ファイルと環境変数から成るアプリケーション設定を
// 読み込み、アクセスする機能を提供します。
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
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

	// TrustedProxies lists the networks the instance's own reverse proxies
	// connect from. A request that arrives from one of them has its client
	// address read from the forwarding header it carries, and a request from
	// anywhere else is attributed to the address that connected. It is empty
	// unless configured, which is what an instance exposed directly needs: a
	// forwarding header is client-supplied input until a proxy known to append
	// to it sits in front.
	//
	// Every hop belongs in the list, not only the nearest one, because the
	// resolution takes the closest address in the chain that is not a listed
	// proxy (see internal/clientip).
	//
	// [Ja] TrustedProxies は、このインスタンス自身のリバースプロキシが接続してくる
	// ネットワークの一覧です。そのいずれかから届いたリクエストは、運んできた転送ヘッダーから
	// クライアントのアドレスを読み取り、それ以外から届いたリクエストは接続してきたアドレスの
	// ものとして扱います。設定しない限り空であり、直接公開されているインスタンスに必要なのは
	// この状態です。転送ヘッダーは、追記すると分かっているプロキシが前段に立つまでは、
	// クライアントが与えた入力に過ぎません。
	//
	// 一覧には最も近い hop だけでなくすべての hop を書きます。解決はチェーンの中で一覧に
	// 無い最も近いアドレスを採るためです (internal/clientip を参照)。
	TrustedProxies []netip.Prefix

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
	// the job instead of at startup. The SMTP settings below are validated at
	// startup instead, because naming that provider is an explicit statement that
	// mail is configured.
	//
	// [Ja] ResendAPIKey / EmailFrom / EmailFromName は送信メールを設定します。ワーカー
	// クライアントがバックグラウンドジョブ用の email sender を構築する際に使い、API キーは
	// EmailProvider が Resend を選んだときにのみ、From アドレスと名前はどちらの
	// プロバイダーでも使います。必須ではなく任意とするのは、メールが未設定のインスタンスでも
	// 起動して設定を仕上げるために到達できる必要があるためです。この場合、送信は起動時では
	// なくジョブ側で失敗します。下の SMTP 設定は代わりに起動時に検証します。そのプロバイダーを
	// 名指しすること自体が、メールを設定済みだという明示的な表明だからです。
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

	// EmailProvider selects which transport delivers outgoing email: Resend's
	// HTTP API or an SMTP relay. It is an explicit setting rather than something
	// inferred from which credentials happen to be present, so that a typo in an
	// SMTP setting surfaces as a startup error naming the missing setting
	// instead of silently leaving the other provider in charge.
	//
	// [Ja] EmailProvider は送信メールをどの transport で配送するか (Resend の HTTP API か
	// SMTP リレーか) を選択します。どの認証情報が設定されているかから推測するのではなく
	// 明示的な設定にしているのは、SMTP の設定のタイプミスが、黙ってもう一方の
	// プロバイダーを使い続ける状態ではなく、不足している設定名を挙げた起動時エラーとして
	// 現れるようにするためです。
	EmailProvider string

	// SMTPHost and SMTPPort address the SMTP relay. The port has no default
	// because the right value follows from the relay's TLS mode (465 and 587 are
	// both common), and a default guessed here would fail at delivery time rather
	// than at startup.
	//
	// [Ja] SMTPHost と SMTPPort は SMTP リレーの宛先です。ポートに既定値を持たせないのは、
	// 適切な値がリレーの TLS モードに従って決まり (465 も 587 も一般的)、ここで推測した
	// 既定値は起動時ではなく配送時に失敗するためです。
	SMTPHost string
	SMTPPort int

	// SMTPUsername and SMTPPassword authenticate to the relay. Both may be empty
	// for a relay that authorises by source address; setting only one is rejected
	// at startup as an incomplete configuration.
	//
	// [Ja] SMTPUsername と SMTPPassword はリレーへの認証情報です。送信元アドレスで
	// 認可するリレーのために両方空にできますが、片方だけの設定は不完全な設定として
	// 起動時に拒否します。
	SMTPUsername string
	SMTPPassword string

	// SMTPTLSMode selects how the connection to the relay is secured. Its values
	// are those of email.SMTPTLSMode, which the worker client converts it to.
	//
	// [Ja] SMTPTLSMode はリレーへの接続の保護方式を選択します。値は email.SMTPTLSMode の
	// ものであり、ワーカークライアントがその型へ変換します。
	SMTPTLSMode string

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

	trustedProxies, err := loadTrustedProxies(file)
	if err != nil {
		return nil, err
	}
	cfg.TrustedProxies = trustedProxies

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

	if err := loadEmailProvider(cfg, file); err != nil {
		return nil, err
	}

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

// loadTrustedProxies reads the networks whose forwarded client address is
// believed, written as a comma-separated list of addresses and CIDR blocks.
// Leaving the setting out is not an error and is the safe state: an instance
// exposed directly has no proxy to trust, and the client address is then the
// one that connected.
//
// An entry that is neither stops startup rather than being dropped. A list that
// silently loses one of its entries resolves the visitors behind that proxy to
// the proxy's own address, and that address is what an audit record — and any
// future per-address limit — is then keyed on.
//
// [Ja] loadTrustedProxies は、転送されたクライアントアドレスを信じる相手のネットワークを
// 読み込みます。アドレスと CIDR ブロックをカンマで区切って並べた形で書きます。設定を書か
// ないことはエラーではなく、それが安全な状態です。直接公開されているインスタンスには信頼
// すべきプロキシが無く、そのときクライアントのアドレスは接続してきたアドレスそのものです。
//
// どちらでもない項目は、取り除かずに起動を止めます。項目を黙って 1 つ落とした一覧は、その
// プロキシの背後にいる訪問者をプロキシ自身のアドレスとして解決してしまい、そのアドレスが
// 監査記録の値となり、将来のアドレス単位の制限のキーにもなるためです。
func loadTrustedProxies(file *fileConfig) ([]netip.Prefix, error) {
	trustedProxies := newSetting("GROOBB_TRUSTED_PROXIES", "server.trusted_proxies", listFileValue(file.Server.TrustedProxies))
	// An array holding only empty strings joins to nothing, which the setting
	// reads as a key nobody wrote. The file is consulted directly here so that
	// those entries reach the check below and are reported as the empty entries
	// they are, rather than disappearing into the same "not configured" that an
	// absent key and an empty array leave behind.
	//
	// [Ja] 空文字列だけを持つ配列は連結すると何も残らず、setting はそれを誰も書いていない
	// キーとして読みます。ここでファイルを直接見るのは、その項目を下の検査に届かせて空の
	// 項目として報告するためです。そうしないと、キーの不在や空の配列と同じ「未設定」へ
	// 消えてしまいます。
	if !trustedProxies.isSet() && len(file.Server.TrustedProxies) == 0 {
		return nil, nil
	}

	entries := strings.Split(trustedProxies.value, ",")
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		prefix, err := ParseTrustedProxy(entry)
		if err != nil {
			return nil, fmt.Errorf("the trusted proxy from %s %s, but is %q", trustedProxies.source(), err, entry)
		}
		prefixes = append(prefixes, prefix)
	}

	return prefixes, nil
}

// ParseTrustedProxy parses one entry of the trusted proxy list, which is either
// a CIDR block or a single address standing for the block that holds only it.
// The prefix is masked so that an entry carrying bits below its prefix length
// ("10.1.2.3/8") is kept in the one form the comparison and the log show.
// An IPv4-mapped IPv6 prefix from /96 through /128 is converted to its IPv4
// equivalent; a shorter prefix is rejected because it also covers addresses
// that cannot be represented by one IPv4 prefix. An IPv6 zone on a single
// address is dropped, because a prefix describes an address range independently
// of an interface and the resolution drops the zone from the peer as well.
//
// It is exported because it is the one place that turns the text an operator
// writes into the form the resolution compares an address against, and the
// resolution's own tests state their trusted proxies as that text. A second
// parser written for those tests would let the two normalisations drift apart,
// which is how an entry that loads but never matches gets through.
//
// [Ja] ParseTrustedProxy は信頼するプロキシの一覧の項目を 1 つ解析します。項目は CIDR
// ブロックか、それ 1 つだけを含むブロックを表す単一のアドレスのいずれかです。prefix 長より
// 下位のビットを持つ項目 ("10.1.2.3/8") が、比較とログが示す 1 つの形に収まるよう、prefix は
// マスクします。/96 から /128 までの IPv4-mapped IPv6 prefix は同等の IPv4 prefix へ
// 変換します。それより短い prefix は、1 つの IPv4 prefix では表現できないアドレスも含むため
// 拒否します。単一のアドレスに付いた IPv6 の zone は取り除きます。prefix はインターフェイスに
// 依存しないアドレス範囲を表し、解決もピアから zone を取り除くためです。
//
// 公開しているのは、運用者が書くテキストを解決が照合する形へ変換する唯一の場所であり、
// 解決自身のテストも信頼するプロキシをそのテキストのまま記述するためです。そのテストのために
// 2 つ目の解析を書くと 2 つの正規化が離れていき、読み込めるのに決して一致しない項目が
// そこをすり抜けます。
func ParseTrustedProxy(entry string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(entry); err == nil {
		if prefix.Addr().Is4In6() {
			if prefix.Bits() < 96 {
				return netip.Prefix{}, errors.New("must use a prefix length from 96 through 128 when written as an IPv4-mapped IPv6 CIDR block")
			}

			addr := prefix.Addr().Unmap()
			return netip.PrefixFrom(addr, prefix.Bits()-96).Masked(), nil
		}

		return prefix.Masked(), nil
	}

	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, errors.New("must be an IP address or a CIDR block")
	}
	addr = addr.Unmap().WithZone("")

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// LogValue renders the configuration for structured logging with its secrets
// replaced, so that logging a Config cannot put the values that authenticate
// the instance into the log. Whether a secret is set is kept, which is what a
// question about the configuration needs to distinguish.
//
// It exists because slog renders a struct field by field: without it, a single
// call that logs a Config anywhere would write out the continuation token key,
// the Resend API key, the SMTP password, and the Turnstile secret key.
//
// [Ja] LogValue は構造化ログ向けに設定を整形し、秘密情報を置き換えます。Config を
// ログに出しても、インスタンスの認証に使う値がログへ入らないようにするためです。
// 秘密情報が設定されているかどうかは残します。設定についての疑問を切り分けるのに
// 必要なのはそこだからです。
//
// これが必要なのは、slog が構造体をフィールドごとに展開するためです。これが無いと、
// どこか 1 箇所で Config をログに渡すだけで、continuation token の鍵・Resend の
// API キー・SMTP のパスワード・Turnstile のシークレットキーが書き出されます。
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", c.Env),
		slog.String("port", c.Port),
		slog.Any("trusted_proxies", c.TrustedProxies),
		slog.String("database_path", c.DatabasePath),
		slog.String("asset_version", c.AssetVersion),
		slog.String("app_url", c.AppURL),
		slog.String("email_provider", c.EmailProvider),
		slog.String("email_from", c.EmailFrom),
		slog.String("email_from_name", c.EmailFromName),
		slog.String("smtp_host", c.SMTPHost),
		slog.Int("smtp_port", c.SMTPPort),
		slog.String("smtp_username", c.SMTPUsername),
		slog.String("smtp_tls_mode", c.SMTPTLSMode),
		slog.String("turnstile_site_key", c.TurnstileSiteKey),
		slog.String("continuation_token_key", redactSecret(c.ContinuationTokenKey)),
		slog.String("resend_api_key", redactSecret(c.ResendAPIKey)),
		slog.String("smtp_password", redactSecret(c.SMTPPassword)),
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

// Email provider values for EmailProvider.
//
// [Ja] EmailProvider が取る送信プロバイダーの値。
const (
	// EmailProviderResend delivers through the Resend HTTP API.
	//
	// [Ja] EmailProviderResend は Resend の HTTP API 経由で配送する。
	EmailProviderResend = "resend"

	// EmailProviderSMTP delivers through an SMTP relay.
	//
	// [Ja] EmailProviderSMTP は SMTP リレー経由で配送する。
	EmailProviderSMTP = "smtp"
)

// SMTP TLS mode values for SMTPTLSMode. They mirror the constants of
// email.SMTPTLSMode, which the worker client converts the setting to. The
// literals are repeated here rather than imported so that config does not
// import the email package: that package pulls in the mail templates, and
// config is imported by nearly every package.
//
// [Ja] SMTPTLSMode が取る TLS モードの値。ワーカークライアントが設定を変換する先である
// email.SMTPTLSMode の定数に対応する。import せずに文字列を再掲しているのは、config が
// email パッケージを import しないようにするため。email パッケージはメールテンプレートを
// 引き込む一方、config はほぼすべてのパッケージから import される。
const (
	smtpTLSModeStartTLS = "starttls"
	smtpTLSModeImplicit = "implicit"
	smtpTLSModeNone     = "none"
)

// loadEmailProvider reads the email provider selection and, when it names SMTP,
// the relay settings that go with it.
//
// [Ja] loadEmailProvider は送信プロバイダーの選択を読み込み、SMTP が指定されている場合は
// それに伴うリレーの設定も読み込む。
func loadEmailProvider(cfg *Config, file *fileConfig) error {
	provider := newSetting("GROOBB_EMAIL_PROVIDER", "email.provider", file.Email.Provider)

	cfg.EmailProvider = provider.value
	if cfg.EmailProvider == "" {
		cfg.EmailProvider = EmailProviderResend
	}

	switch cfg.EmailProvider {
	case EmailProviderResend:
		return nil
	case EmailProviderSMTP:
		return loadSMTPSettings(cfg, file)
	default:
		return fmt.Errorf("the email provider from %s must be %q or %q, but is %q", provider.source(), EmailProviderResend, EmailProviderSMTP, cfg.EmailProvider)
	}
}

// loadSMTPSettings reads and validates the SMTP relay settings. Every missing or
// malformed value is reported as a startup error naming the setting: an operator
// configuring a relay has stated that mail should work, so a setting they got
// wrong must stop the instance rather than surface later as mail that silently
// never arrives.
//
// [Ja] loadSMTPSettings は SMTP リレーの設定を読み込んで検証する。欠落や不正な値は
// すべて、設定名を挙げた起動時エラーとして報告する。リレーを設定した運用者はメールが
// 機能すべきだと表明しているのだから、設定の誤りは、後になって「メールが黙って届かない」
// 形で表面化するのではなく、インスタンスを止めなければならない。
func loadSMTPSettings(cfg *Config, file *fileConfig) error {
	smtpConfigured := fmt.Sprintf("the email provider is %q", EmailProviderSMTP)

	host := newSetting("GROOBB_SMTP_HOST", "email.smtp.host", file.Email.SMTP.Host)
	if !host.isSet() {
		return host.missingWhenError(smtpConfigured)
	}
	cfg.SMTPHost = host.value

	port := newSetting("GROOBB_SMTP_PORT", "email.smtp.port", intFileValue(file.Email.SMTP.Port))
	if !port.isSet() {
		return port.missingWhenError(smtpConfigured)
	}
	parsedPort, err := port.tcpPort("SMTP port")
	if err != nil {
		return err
	}
	cfg.SMTPPort = parsedPort

	username := newSetting("GROOBB_SMTP_USERNAME", "email.smtp.username", file.Email.SMTP.Username)
	password := newSetting("GROOBB_SMTP_PASSWORD", "email.smtp.password", file.Email.SMTP.Password)
	if username.isSet() != password.isSet() {
		return fmt.Errorf("%s and %s must be set together, or both left unset", username.names(), password.names())
	}
	cfg.SMTPUsername = username.value
	cfg.SMTPPassword = password.value

	tlsMode := newSetting("GROOBB_SMTP_TLS_MODE", "email.smtp.tls_mode", file.Email.SMTP.TLSMode)
	cfg.SMTPTLSMode = tlsMode.value
	if cfg.SMTPTLSMode == "" {
		cfg.SMTPTLSMode = smtpTLSModeStartTLS
	}
	switch cfg.SMTPTLSMode {
	case smtpTLSModeStartTLS, smtpTLSModeImplicit, smtpTLSModeNone:
	default:
		return fmt.Errorf("the SMTP TLS mode from %s must be %q, %q, or %q, but is %q", tlsMode.source(), smtpTLSModeStartTLS, smtpTLSModeImplicit, smtpTLSModeNone, cfg.SMTPTLSMode)
	}

	// The From address has no fallback in SMTP: an empty envelope sender is what
	// bounce notifications use, and relays reject mail sent with one.
	//
	// [Ja] SMTP では From アドレスに代替が無い。空のエンベロープ送信者はバウンス通知が
	// 使うものであり、それで送られたメールをリレーは拒否する。
	if cfg.EmailFrom == "" {
		return newSetting("GROOBB_EMAIL_FROM", "email.from", file.Email.From).missingWhenError(smtpConfigured)
	}

	// An unencrypted relay is legitimate on a trusted local channel, so this is a
	// warning rather than a startup error; in production it is worth saying out
	// loud, because the mail Groobb sends carries sign-in and password-reset
	// links.
	//
	// [Ja] 暗号化しないリレーは信頼できるローカル経路では正当なため、起動時エラーではなく
	// 警告に留める。ただし本番では明示的に言う価値がある。Groobb が送るメールはサインインや
	// パスワードリセットのリンクを運ぶためである。
	if cfg.IsProduction() && cfg.SMTPTLSMode == smtpTLSModeNone {
		slog.Warn("SMTP の TLS を無効にしています (本番でメールの内容が平文で流れます)")
	}

	return nil
}
