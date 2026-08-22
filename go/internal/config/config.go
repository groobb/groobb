// Package config provides loading and access to application settings from
// environment variables.
//
// [Ja] config パッケージは、環境変数からアプリケーション設定を読み込み、
// アクセスする機能を提供します。
package config

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

	// AssetVersion is the cache-busting value (a Git commit hash) used for
	// static assets in non-dev environments. In dev a timestamp is used instead;
	// see GetAssetVersion.
	//
	// [Ja] AssetVersion は非開発環境で静的アセットに使うキャッシュ無効化用の値
	// (Git コミットハッシュ) です。開発環境では代わりにタイムスタンプを使います
	// (GetAssetVersion を参照)。
	AssetVersion string

	// ResendAPIKey, EmailFrom, and EmailFromName configure outgoing email through
	// Resend. They are consumed by the worker client when it builds the email
	// sender for background jobs. They are optional rather than required: the
	// worker is not started yet (its first enqueue-side consumer, sign-up, comes
	// in a later task), so a deployment without email configured must still boot.
	// When the worker is wired into main.go, missing values can be promoted to a
	// startup error then.
	//
	// [Ja] ResendAPIKey / EmailFrom / EmailFromName は Resend 経由の送信メールを設定
	// します。ワーカークライアントがバックグラウンドジョブ用の email sender を構築する
	// 際に使います。必須ではなく任意とします。ワーカーはまだ起動されておらず (最初の
	// 投入側の利用者であるサインアップは後続タスク)、メール未設定のデプロイでも起動できる
	// 必要があるためです。ワーカーを main.go に配線する時点で、欠落を起動時エラーへ
	// 格上げできます。
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

// Load reads the configuration from environment variables.
//
// Every environment expects the variables to be already exported into the
// process before startup: locally `op run --env-file=.env` resolves them, and
// in CI / production the runtime sets them directly.
//
// [Ja] Load は環境変数から設定を読み込みます。
//
// いずれの環境でも、プロセス起動時には環境変数が既にエクスポートされている
// ことを前提とします。ローカルでは `op run --env-file=.env` が解決し、
// CI / 本番ではランタイムが直接設定します。
func Load() (*Config, error) {
	// APP_ENV defaults to "dev" when unset.
	//
	// [Ja] APP_ENV は未設定の場合 "dev" を既定値とします。
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	cfg := &Config{
		Env: env,
	}

	cfg.Port = os.Getenv("GROOBB_PORT")
	if cfg.Port == "" {
		return nil, fmt.Errorf("required environment variable GROOBB_PORT is not set")
	}

	cfg.DatabasePath = os.Getenv("GROOBB_DATABASE_PATH")
	if cfg.DatabasePath == "" {
		return nil, fmt.Errorf("required environment variable GROOBB_DATABASE_PATH is not set")
	}

	cfg.ContinuationTokenKey = os.Getenv("GROOBB_CONTINUATION_TOKEN_KEY")
	if cfg.ContinuationTokenKey == "" {
		return nil, fmt.Errorf("required environment variable GROOBB_CONTINUATION_TOKEN_KEY is not set")
	}
	if len(cfg.ContinuationTokenKey) < ContinuationTokenMinimumKeyLength {
		return nil, fmt.Errorf("GROOBB_CONTINUATION_TOKEN_KEY must be at least %d bytes", ContinuationTokenMinimumKeyLength)
	}

	// Pin the asset version to the current commit so that non-dev environments
	// serve stable, cache-busting asset URLs per deploy.
	//
	// [Ja] 非開発環境がデプロイ単位で安定したキャッシュ無効化 URL を配信できるよう、
	// アセットバージョンを現在のコミットに固定します。
	cfg.AssetVersion = getGitCommitHash()

	// Email settings are read without requiring them: the worker that uses them
	// is not started yet, so a deployment without email configured must still
	// boot (see the field docs).
	//
	// [Ja] メール設定は必須にせず読み込む。これらを使うワーカーはまだ起動されない
	// ため、メール未設定のデプロイでも起動できる必要がある (フィールドのドキュメントを
	// 参照)。
	cfg.ResendAPIKey = os.Getenv("GROOBB_RESEND_API_KEY")
	cfg.EmailFrom = os.Getenv("GROOBB_EMAIL_FROM")
	cfg.EmailFromName = os.Getenv("GROOBB_EMAIL_FROM_NAME")

	// AppURL is read without requiring it, for the same reason as the email
	// settings above (see the field docs).
	//
	// [Ja] AppURL は必須にせず読み込む。理由は上のメール設定と同じ (フィールドの
	// ドキュメントを参照)。
	cfg.AppURL = os.Getenv("GROOBB_APP_URL")

	// Turnstile keys are read without requiring them: Turnstile is enabled
	// operationally by provisioning the real keys, so a deployment must still boot
	// before they are set (see the field docs).
	//
	// [Ja] Turnstile キーは必須にせず読み込む。Turnstile は実キーを設定する運用手順で
	// 有効化するため、キー設定前でもデプロイが起動できる必要がある (フィールドの
	// ドキュメントを参照)。
	cfg.TurnstileSiteKey = os.Getenv("GROOBB_TURNSTILE_SITE_KEY")
	cfg.TurnstileSecretKey = os.Getenv("GROOBB_TURNSTILE_SECRET_KEY")

	// GROOBB_TURNSTILE_DISABLE lets non-production environments switch Turnstile
	// off with a single flag instead of unsetting both keys. When enabled, both
	// keys are cleared so the empty-key path takes over: token verification is
	// bypassed and the widget is not rendered.
	//
	// In production the flag is deliberately ignored (fail-closed) and only logged
	// as a warning. Turnstile is bot protection, so a stray
	// GROOBB_TURNSTILE_DISABLE=true leaking into production must never silently
	// disable it.
	//
	// [Ja] GROOBB_TURNSTILE_DISABLE は、非本番環境で 2 つのキーを未設定にする代わりに
	// 1 フラグで Turnstile を無効化するためのものです。有効時は両キーを空に落とし、
	// キー空の経路 (トークン検証がバイパスされ、ウィジェットも描画されない) に
	// 委ねます。
	//
	// 本番ではフラグを意図的に無視し (fail-closed)、warn ログを出すだけにします。
	// Turnstile は Bot 対策なので、GROOBB_TURNSTILE_DISABLE=true が誤って本番に漏れても、
	// 黙って無効化されてはなりません。
	if os.Getenv("GROOBB_TURNSTILE_DISABLE") == "true" {
		if cfg.IsProduction() {
			slog.Warn("GROOBB_TURNSTILE_DISABLE は本番環境では無視されます (Bot 対策を維持するため fail-closed)")
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

// getGitCommitHash returns the short hash of the current Git commit, or "dev"
// as a fallback when Git is unavailable (e.g. a binary running outside a repo).
//
// [Ja] getGitCommitHash は現在の Git コミットの短縮ハッシュを返します。Git が
// 使えない場合 (例: リポジトリ外で動くバイナリ) はフォールバックとして "dev" を
// 返します。
func getGitCommitHash() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}
