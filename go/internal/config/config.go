// configパッケージは、TOMLの設定ファイルと環境変数から成るアプリケーション設定を
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

// ContinuationTokenMinimumKeyLengthは受け付けるContinuationTokenKeyの最小長
// (バイト) です。32バイトはSHA-256の256 bitの出力長に対応します。設定値自体は
// 暗号学的に安全な乱数源から生成する必要があり、この長さ検査は短い鍵を拒否しますが、
// エントロピーまでは保証できません。
//
// 署名処理の隣ではなくここに置くのは、Loadが起動時に短い鍵を拒否し、署名処理側は短い鍵で
// fail-closedになるという2つの判定が同じ値を見る必要があるためです。片方だけを引き上げ
// ると、アプリケーションは起動するのに何も署名できない鍵が生まれます。
const ContinuationTokenMinimumKeyLength = 32

// Configはアプリケーションの設定を保持します。
type Config struct {
	// Envは実行環境 ("dev" / "test" / "prod") を表します。
	Env string

	// ContinuationTokenKeyはメール確認と2段階認証サインインの各ステップ間で
	// サーバー側状態を運ぶ短命Cookieに署名します。32バイト以上の安定した秘密値で
	// なければなりません。変更すると発行済みcontinuation tokenが無効になり、漏えいすると
	// 攻撃者が認証状態を偽造できるためです。
	ContinuationTokenKey string

	// DatabasePathはSQLiteデータベースファイルのファイルシステム上のパスです。
	// インスタンスが保持する状態はすべてこのファイルにあるため、既定値を持たせず必須と
	// します。それらしい既定値を置くと、設定を誤ったインスタンスが「データの在り処が
	// 分からない」と報告する代わりに、自分で作った空のデータベースの上で起動して
	// しまうためです。
	DatabasePath string

	// PortはHTTPサーバーが待ち受けるTCPポートです。
	Port string

	// TrustedProxiesは、このインスタンス自身のリバースプロキシが接続してくる
	// ネットワークの一覧です。そのいずれかから届いたリクエストは、運んできた転送ヘッダーから
	// クライアントのアドレスを読み取り、それ以外から届いたリクエストは接続してきたアドレスの
	// ものとして扱います。設定しない限り空であり、直接公開されているインスタンスに必要なのは
	// この状態です。転送ヘッダーは、追記すると分かっているプロキシが前段に立つまでは、
	// クライアントが与えた入力に過ぎません。
	//
	// 一覧には最も近いhopだけでなくすべてのhopを書きます。解決はチェーンの中で一覧に
	// 無い最も近いアドレスを採るためです (internal/clientipを参照)。
	TrustedProxies []netip.Prefix

	// AssetVersionは非開発環境で静的アセットに使うキャッシュ無効化用の値です。
	// ビルド時にバイナリへ埋め込まれた値から起動時に固定します (buildAssetVersionを
	// 参照)。開発環境では代わりにタイムスタンプを使います (GetAssetVersionを参照)。
	AssetVersion string

	// ResendAPIKey / EmailFrom / EmailFromNameは送信メールを設定します。ワーカー
	// クライアントがバックグラウンドジョブ用のemail senderを構築する際に使い、APIキーは
	// EmailProviderがResendを選んだときにのみ、Fromアドレスと名前はどちらの
	// プロバイダーでも使います。必須ではなく任意とするのは、メールが未設定のインスタンスでも
	// 起動して設定を仕上げるために到達できる必要があるためです。この場合、送信は起動時では
	// なくジョブ側で失敗します。下のSMTP設定は代わりに起動時に検証します。そのプロバイダーを
	// 名指しすること自体が、メールを設定済みだという明示的な表明だからです。
	ResendAPIKey string

	// EmailFromは送信メールのFromヘッダーに使う送信元アドレスです。
	EmailFrom string

	// EmailFromNameはFromヘッダーでEmailFromと並べて表示する送信元の表示名
	// です。
	EmailFromName string

	// EmailProviderは送信メールをどのtransportで配送するか (ResendのHTTP APIか
	// SMTPリレーか) を選択します。どの認証情報が設定されているかから推測するのではなく
	// 明示的な設定にしているのは、SMTPの設定のタイプミスが、黙ってもう一方の
	// プロバイダーを使い続ける状態ではなく、不足している設定名を挙げた起動時エラーとして
	// 現れるようにするためです。
	EmailProvider string

	// SMTPHostとSMTPPortはSMTPリレーの宛先です。ポートに既定値を持たせないのは、
	// 適切な値がリレーのTLSモードに従って決まり (465も587も一般的)、ここで推測した
	// 既定値は起動時ではなく配送時に失敗するためです。
	SMTPHost string
	SMTPPort int

	// SMTPUsernameとSMTPPasswordはリレーへの認証情報です。送信元アドレスで
	// 認可するリレーのために両方空にできますが、片方だけの設定は不完全な設定として
	// 起動時に拒否します。
	SMTPUsername string
	SMTPPassword string

	// SMTPTLSModeはリレーへの接続の保護方式を選択します。値はemail.SMTPTLSModeの
	// ものであり、ワーカークライアントがその型へ変換します。
	SMTPTLSMode string

	// AppURLはアプリケーションの公開ベースURL (例: 本番は
	// "https://groobb.example.dev"、devは "http://localhost:8080") で、スキームとホスト
	// だけを持ち、パスも末尾スラッシュも付けません。インスタンスが公開する絶対アドレスは
	// すべてこの値の下で組み立てられます。送信メール内のリンク (パスワードリセットリンク
	// など) と、公開ページが自身を宣言するcanonical URL・BreadcrumbListの構造化データが
	// それです。いずれもそれが書かれた文書から離れて読まれるため、相対パスでは表現でき
	// ません。素のドメインではなくベースURL全体を保持するのは、dev (ポート上の平文HTTP)
	// が要求するスキームとポートを含められるようにするためです。
	//
	// メール設定と同じ理由で必須ではなく任意とし (メール未設定のデプロイでも起動できる
	// 必要があるため)、指定された値は起動時に検証します (httpBaseURLを参照)。空のときに
	// 残るものは呼び出し側で異なります。メールのリンクはホスト相対のURLになり、公開ページ
	// は解決できないアドレスを名指す代わりにcanonicalのリンクも構造化データも描画しません。
	AppURL string

	// TurnstileSiteKeyとTurnstileSecretKeyは公開フォームのCloudflare
	// TurnstileによるBot対策を設定します。サイトキーはウィジェットを描画するために
	// テンプレートへ渡し、シークレットキーは送信されたトークンをサーバー側で検証するのに
	// 使います。任意であり、両方が空のとき (dev / testの無効化時) はウィジェットを描画せず、
	// トークン検証もバイパスします。
	TurnstileSiteKey   string
	TurnstileSecretKey string
}

// LoadはTOMLの設定ファイルと環境変数から設定を読み込みます。設定ごとに、
// 環境変数がファイルより優先されます。
//
// 設定ファイルは任意です。ファイルが無ければすべての設定は環境変数から読み込まれ、
// 開発環境 (`op run --env-file=.env` が値を解決する) とCIはこの形で動きます。
// セルフホストのインスタンスは設定をファイルに置き、環境変数は個別の上書きや、
// デプロイ側が注入する秘密情報のために使うことを想定します。
func Load() (*Config, error) {
	file, err := loadFile()
	if err != nil {
		return nil, err
	}

	// 実行環境はどちらの入力も設定していない場合 "dev" を既定値とします。
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

	// 非開発環境がデプロイの間ずっと安定したキャッシュ無効化URLを配信できるよう、
	// アセットバージョンを起動時に一度だけ固定します。
	cfg.AssetVersion = buildAssetVersion(assetVersion, vcsRevision())

	// メール設定は必須にせず読み込む。メールが未設定のインスタンスでも、設定を
	// 仕上げるために到達できるよう起動できる必要がある (フィールドのドキュメントを
	// 参照)。
	cfg.ResendAPIKey = newSetting("GROOBB_RESEND_API_KEY", "email.resend_api_key", file.Email.ResendAPIKey).value
	cfg.EmailFrom = newSetting("GROOBB_EMAIL_FROM", "email.from", file.Email.From).value
	cfg.EmailFromName = newSetting("GROOBB_EMAIL_FROM_NAME", "email.from_name", file.Email.FromName).value

	if err := loadEmailProvider(cfg, file); err != nil {
		return nil, err
	}

	// AppURLは必須にせず読み込む。理由は上のメール設定と同じですが、指定された値は
	// URLの組み立てに届く前に検証します (フィールドのドキュメントを参照)。
	appURL := newSetting("GROOBB_APP_URL", "app.url", file.App.URL)
	cfg.AppURL, err = appURL.httpBaseURL("application URL")
	if err != nil {
		return nil, err
	}

	// Turnstileキーは必須にせず読み込む。Turnstileは実キーを設定する運用手順で
	// 有効化するため、キー設定前でもデプロイが起動できる必要がある (フィールドの
	// ドキュメントを参照)。
	cfg.TurnstileSiteKey = newSetting("GROOBB_TURNSTILE_SITE_KEY", "turnstile.site_key", file.Turnstile.SiteKey).value
	cfg.TurnstileSecretKey = newSetting("GROOBB_TURNSTILE_SECRET_KEY", "turnstile.secret_key", file.Turnstile.SecretKey).value

	// 無効化フラグは、非本番環境で2つのキーを未設定にする代わりに1つの設定で
	// Turnstileを無効化するためのものです。有効時は両キーを空に落とし、キー空の経路
	// (トークン検証がバイパスされ、ウィジェットも描画されない) に委ねます。
	//
	// 本番ではフラグを意図的に無視し (fail-closed)、warnログを出すだけにします。
	// TurnstileはBot対策なので、無効化フラグが誤って本番に漏れても、黙って無効化されては
	// なりません。
	if newSetting("GROOBB_TURNSTILE_DISABLE", "turnstile.disable", boolFileValue(file.Turnstile.Disable)).value == "true" {
		if cfg.IsProduction() {
			slog.Warn("Turnstileの無効化設定は本番環境では無視されます (Bot対策を維持するためfail-closed)")
		} else {
			cfg.TurnstileSiteKey = ""
			cfg.TurnstileSecretKey = ""
		}
	}

	// 本番では2つのキーのうち片方だけが設定されているときに警告します。
	// シークレットキーのないサイトキーは、ウィジェットを描画しつつサーバー側の検証を
	// バイパスする (シークレット空) ため、有効に見えてBot対策が黙って無効になります。
	// キーは任意 (設定前でもデプロイが起動できる必要がある) なので起動時エラーにはせず
	// 警告に留めます。両キーが空の状態は「まだ導入していない」意図的な状態として警告しません。
	if cfg.IsProduction() && (cfg.TurnstileSiteKey == "") != (cfg.TurnstileSecretKey == "") {
		slog.Warn("Turnstileのキーが片方のみ設定されています (本番でBot対策が黙って無効化される恐れ)")
	}

	return cfg, nil
}

// loadTrustedProxiesは、転送されたクライアントアドレスを信じる相手のネットワークを
// 読み込みます。アドレスとCIDRブロックをカンマで区切って並べた形で書きます。設定を書か
// ないことはエラーではなく、それが安全な状態です。直接公開されているインスタンスには信頼
// すべきプロキシが無く、そのときクライアントのアドレスは接続してきたアドレスそのものです。
//
// どちらでもない項目は、取り除かずに起動を止めます。項目を黙って1つ落とした一覧は、その
// プロキシの背後にいる訪問者をプロキシ自身のアドレスとして解決してしまい、そのアドレスが
// 監査記録の値となり、将来のアドレス単位の制限のキーにもなるためです。
func loadTrustedProxies(file *fileConfig) ([]netip.Prefix, error) {
	trustedProxies := newSetting("GROOBB_TRUSTED_PROXIES", "server.trusted_proxies", listFileValue(file.Server.TrustedProxies))
	// 空文字列だけを持つ配列は連結すると何も残らず、settingはそれを誰も書いていない
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

// ParseTrustedProxyは信頼するプロキシの一覧の項目を1つ解析します。項目はCIDR
// ブロックか、それ1つだけを含むブロックを表す単一のアドレスのいずれかです。prefix長より
// 下位のビットを持つ項目 ("10.1.2.3/8") が、比較とログが示す1つの形に収まるよう、prefixは
// マスクします。/96から /128までのIPv4-mapped IPv6 prefixは同等のIPv4 prefixへ
// 変換します。それより短いprefixは、1つのIPv4 prefixでは表現できないアドレスも含むため
// 拒否します。単一のアドレスに付いたIPv6のzoneは取り除きます。prefixはインターフェイスに
// 依存しないアドレス範囲を表し、解決もピアからzoneを取り除くためです。
//
// 公開しているのは、運用者が書くテキストを解決が照合する形へ変換する唯一の場所であり、
// 解決自身のテストも信頼するプロキシをそのテキストのまま記述するためです。そのテストのために
// 2つ目の解析を書くと2つの正規化が離れていき、読み込めるのに決して一致しない項目が
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

// LogValueは構造化ログ向けに設定を整形し、秘密情報を置き換えます。Configを
// ログに出しても、インスタンスの認証に使う値がログへ入らないようにするためです。
// 秘密情報が設定されているかどうかは残します。設定についての疑問を切り分けるのに
// 必要なのはそこだからです。
//
// これが必要なのは、slogが構造体をフィールドごとに展開するためです。これが無いと、
// どこか1箇所でConfigをログに渡すだけで、continuation tokenの鍵・Resendの
// APIキー・SMTPのパスワード・Turnstileのシークレットキーが書き出されます。
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

// redactedSecretは、設定済みの秘密情報の代わりにLogValueが表示する値です。
const redactedSecret = "[REDACTED]"

// redactSecretは秘密情報をマーカーに置き換えます。未設定のものは空のままにし、
// どの秘密情報が欠けているかはログから分かるようにします。
func redactSecret(secret string) string {
	if secret == "" {
		return ""
	}

	return redactedSecret
}

// IsDevは実行環境が開発環境かどうかを返します。
func (c *Config) IsDev() bool {
	return c.Env == "dev"
}

// IsTestは実行環境がテスト環境かどうかを返します。
func (c *Config) IsTest() bool {
	return c.Env == "test"
}

// IsProductionは実行環境が本番環境かどうかを返します。
func (c *Config) IsProduction() bool {
	return c.Env == "prod"
}

// GetAssetVersionは静的アセットのURLに付与するキャッシュ無効化用の値を
// 返します。
//
// 開発環境では呼び出しごとに新しいミリ秒タイムスタンプを返し、CSS / JSの編集を
// 手動のキャッシュクリアなしに反映できるようにします。それ以外の環境では、起動時に
// 固定した静的なAssetVersionを返します。
func (c *Config) GetAssetVersion() string {
	if c.IsDev() {
		return strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	return c.AssetVersion
}

// assetVersionはビルド時に
// `-ldflags "-X github.com/groobb/groobb/go/internal/config.assetVersion=..."`
// でバイナリへ埋め込まれます (go/Makefileのbuildターゲットを参照)。リンカが書き込め
// るのはパッケージレベルの変数だけのためこの形とし、フラグ無しのビルドが下のリビジョンへ
// フォールバックできるよう空のままにしています。
var assetVersion string

// buildAssetVersionは、ビルド時に埋め込まれた値からアセットバージョンを決め、
// 無ければバイナリのビルド元リビジョン、最後に "dev" へフォールバックします。
//
// フォールバックが重要なのは、変化しない値が古いCSSを配信するためです。ソースから素の
// `go build` でビルドする運用者には値が埋め込まれず、そうしたビルドをすべて1つの定数に
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

// vcsRevisionはバイナリのビルド元となった完全なリビジョンを返します。ビルドが
// バージョン管理の情報を持たない場合 (ソースアーカイブからのビルドやテストバイナリ) は
// 空文字列を返します。完全な値を保つことでlinker stampと揃え、短縮した接頭辞の衝突に
// よって長期のアセットキャッシュキーが再利用されないようにします。
//
// `git` を実行せずツールチェインが記録したビルド情報を読むのは、配布されたバイナリの
// 周囲には実行時にリポジトリもgitの実行ファイルも無いためです。
func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	return vcsRevisionFromSettings(info.Settings)
}

// vcsRevisionFromSettingsはビルド設定から完全なリビジョンを返し、リビジョン設定が
// 無い場合は空文字列を返します。抽出をdebug.ReadBuildInfoから分けることで、すべての
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

// EmailProviderが取る送信プロバイダーの値。
const (
	// EmailProviderResendはResendのHTTP API経由で配送する。
	EmailProviderResend = "resend"

	// EmailProviderSMTPはSMTPリレー経由で配送する。
	EmailProviderSMTP = "smtp"
)

// SMTPTLSModeが取るTLSモードの値。ワーカークライアントが設定を変換する先である
// email.SMTPTLSModeの定数に対応する。importせずに文字列を再掲しているのは、configが
// emailパッケージをimportしないようにするため。emailパッケージはメールテンプレートを
// 引き込む一方、configはほぼすべてのパッケージからimportされる。
const (
	smtpTLSModeStartTLS = "starttls"
	smtpTLSModeImplicit = "implicit"
	smtpTLSModeNone     = "none"
)

// loadEmailProviderは送信プロバイダーの選択を読み込み、SMTPが指定されている場合は
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

// loadSMTPSettingsはSMTPリレーの設定を読み込んで検証する。欠落や不正な値は
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

	// SMTPではFromアドレスに代替が無い。空のエンベロープ送信者はバウンス通知が
	// 使うものであり、それで送られたメールをリレーは拒否する。
	if cfg.EmailFrom == "" {
		return newSetting("GROOBB_EMAIL_FROM", "email.from", file.Email.From).missingWhenError(smtpConfigured)
	}

	// 暗号化しないリレーは信頼できるローカル経路では正当なため、起動時エラーではなく
	// 警告に留める。ただし本番では明示的に言う価値がある。Groobbが送るメールはサインインや
	// パスワードリセットのリンクを運ぶためである。
	if cfg.IsProduction() && cfg.SMTPTLSMode == smtpTLSModeNone {
		slog.Warn("SMTPのTLSを無効にしています (本番でメールの内容が平文で流れます)")
	}

	return nil
}
