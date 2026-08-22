package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fullConfigFile sets every setting, so that a case can assert the file is read
// as a whole rather than only the settings it happens to name.
//
// [Ja] fullConfigFile はすべての設定を書いたファイルです。ケースが名指しした設定だけで
// なく、ファイル全体が読まれることを検証できるようにするためです。
const fullConfigFile = `
[app]
env = "prod"
url = "https://groobb.example.dev"

[server]
port = 9090
trusted_proxies = ["127.0.0.1", "10.0.0.0/8"]

[database]
path = "/var/lib/groobb/groobb.sqlite"

[security]
continuation_token_key = "groobb-test-continuation-token-key-32-bytes"

[email]
provider = "smtp"
from = "noreply@example.dev"
from_name = "Groobb"
resend_api_key = "re_test_key"

[email.smtp]
host = "smtp.example.dev"
port = 587
username = "smtp-user"
password = "smtp-password"
tls_mode = "implicit"

[turnstile]
site_key = "1x00000000000000000000AA"
secret_key = "1x0000000000000000000000000000000AA"
disable = false
`

// writeConfigFile writes a configuration file at the default path, inside the
// working directory clearEnv moved the test to.
//
// [Ja] writeConfigFile は、clearEnv がテストを移した作業ディレクトリの中に、既定の
// パスで設定ファイルを書き出します。
func writeConfigFile(t *testing.T, contents string) {
	t.Helper()

	if err := os.WriteFile(defaultConfigFileName, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write the configuration file: %v", err)
	}
}

// TestLoadReadsSettingsFromConfigFile verifies that a file alone configures an
// instance, which is what a self-hosted deployment relies on.
//
// [Ja] TestLoadReadsSettingsFromConfigFile は、ファイルだけでインスタンスを設定できる
// ことを検証する。セルフホストのデプロイが頼りにするのはこの形であるため。
func TestLoadReadsSettingsFromConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, fullConfigFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Env", got: cfg.Env, want: "prod"},
		{name: "AppURL", got: cfg.AppURL, want: "https://groobb.example.dev"},
		{name: "Port", got: cfg.Port, want: "9090"},
		{name: "DatabasePath", got: cfg.DatabasePath, want: "/var/lib/groobb/groobb.sqlite"},
		{name: "ContinuationTokenKey", got: cfg.ContinuationTokenKey, want: "groobb-test-continuation-token-key-32-bytes"},
		{name: "EmailProvider", got: cfg.EmailProvider, want: EmailProviderSMTP},
		{name: "EmailFrom", got: cfg.EmailFrom, want: "noreply@example.dev"},
		{name: "EmailFromName", got: cfg.EmailFromName, want: "Groobb"},
		{name: "ResendAPIKey", got: cfg.ResendAPIKey, want: "re_test_key"},
		{name: "SMTPHost", got: cfg.SMTPHost, want: "smtp.example.dev"},
		{name: "SMTPUsername", got: cfg.SMTPUsername, want: "smtp-user"},
		{name: "SMTPPassword", got: cfg.SMTPPassword, want: "smtp-password"},
		{name: "SMTPTLSMode", got: cfg.SMTPTLSMode, want: smtpTLSModeImplicit},
		{name: "TurnstileSiteKey", got: cfg.TurnstileSiteKey, want: turnstileTestSiteKey},
		{name: "TurnstileSecretKey", got: cfg.TurnstileSecretKey, want: turnstileTestSecretKey},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}

	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want 587", cfg.SMTPPort)
	}

	// The trusted proxies are a list rather than a scalar, so they are checked
	// apart from the table above. Their rendered form is what the resolution
	// compares an address against.
	//
	// [Ja] 信頼するプロキシはスカラーではなくリストのため、上の表とは別に検証する。
	// 整形した形は、解決がアドレスを照合する相手そのものである。
	wantTrustedProxies := []string{"127.0.0.1/32", "10.0.0.0/8"}
	gotTrustedProxies := make([]string, 0, len(cfg.TrustedProxies))
	for _, prefix := range cfg.TrustedProxies {
		gotTrustedProxies = append(gotTrustedProxies, prefix.String())
	}
	if !slices.Equal(gotTrustedProxies, wantTrustedProxies) {
		t.Errorf("TrustedProxies = %v, want %v", gotTrustedProxies, wantTrustedProxies)
	}
}

// TestLoadRejectsAnInvalidTrustedProxyFromTheConfigFile verifies that a bad
// entry in the file is reported the same way as one in the environment, naming
// the file key rather than the variable, so that the operator looks where they
// wrote it.
//
// [Ja] TestLoadRejectsAnInvalidTrustedProxyFromTheConfigFile は、ファイルの不正な項目が
// 環境変数のものと同じように報告され、変数ではなくファイルのキーを挙げることを検証する。
// 運用者が自分の書いた場所を見に行けるようにするためである。
func TestLoadRejectsAnInvalidTrustedProxyFromTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(fullConfigFile, `trusted_proxies = ["127.0.0.1", "10.0.0.0/8"]`, `trusted_proxies = ["proxy.example.dev"]`, 1))

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should reject a trusted proxy that is not an address")
	}
	if !strings.Contains(err.Error(), "server.trusted_proxies") {
		t.Errorf("the error should name the file key the value came from, got: %v", err)
	}
}

// TestLoadRejectsAnEmptyTrustedProxyFromTheConfigFile verifies that an array of
// empty strings is reported rather than read as a key nobody wrote. An entry
// left empty is the same mistake whether it is the only one or one of several,
// so the operator hears about it either way.
//
// [Ja] TestLoadRejectsAnEmptyTrustedProxyFromTheConfigFile は、空文字列だけの配列が、
// 誰も書いていないキーとして読まれるのではなく報告されることを検証する。空のまま残った項目は、
// それが唯一の項目でも複数のうちの 1 つでも同じ誤りなので、どちらでも運用者に伝わるように
// する。
func TestLoadRejectsAnEmptyTrustedProxyFromTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(fullConfigFile, `trusted_proxies = ["127.0.0.1", "10.0.0.0/8"]`, `trusted_proxies = [""]`, 1))

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should reject a trusted proxy list holding an empty entry")
	}
	if !strings.Contains(err.Error(), "server.trusted_proxies") {
		t.Errorf("the error should name the file key the value came from, got: %v", err)
	}
}

// TestLoadPrefersTheEnvironmentOverTheConfigFile verifies that a setting given
// in both places takes the value from the environment, and that the settings
// the environment leaves alone keep the file's values.
//
// [Ja] TestLoadPrefersTheEnvironmentOverTheConfigFile は、両方に書かれた設定が環境変数の
// 値になり、環境変数が触れていない設定はファイルの値を保つことを検証する。
func TestLoadPrefersTheEnvironmentOverTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, fullConfigFile)
	t.Setenv("GROOBB_PORT", "8080")
	t.Setenv("GROOBB_SMTP_PASSWORD", "smtp-password-from-the-environment")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want the value from the environment %q", cfg.Port, "8080")
	}
	if cfg.SMTPPassword != "smtp-password-from-the-environment" {
		t.Error("SMTPPassword should come from the environment when it is set there")
	}
	if cfg.SMTPUsername != "smtp-user" {
		t.Errorf("SMTPUsername = %q, want the value from the file %q", cfg.SMTPUsername, "smtp-user")
	}
}

// TestLoadWithoutAConfigFile verifies that a missing default file is not an
// error, so an instance configured entirely through the environment starts.
//
// [Ja] TestLoadWithoutAConfigFile は、既定のファイルが無いことがエラーにならず、環境変数
// だけで設定したインスタンスが起動することを検証する。
func TestLoadWithoutAConfigFile(t *testing.T) {
	setRequiredEnv(t)

	if _, err := os.Stat(defaultConfigFileName); !os.IsNotExist(err) {
		t.Fatalf("the working directory should hold no configuration file, got %v", err)
	}

	if _, err := Load(); err != nil {
		t.Fatalf("Load() should not fail without a configuration file: %v", err)
	}
}

// TestLoadReadsTheConfigFileNamedByTheEnvironment verifies that a file outside
// the working directory is read when the operator points at it.
//
// [Ja] TestLoadReadsTheConfigFileNamedByTheEnvironment は、運用者が指定したときに作業
// ディレクトリの外にあるファイルが読まれることを検証する。
func TestLoadReadsTheConfigFileNamedByTheEnvironment(t *testing.T) {
	clearEnv(t)

	path := filepath.Join(t.TempDir(), "instance.toml")
	if err := os.WriteFile(path, []byte(fullConfigFile), 0o600); err != nil {
		t.Fatalf("failed to write the configuration file: %v", err)
	}
	t.Setenv(configFileEnvName, path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q from the file named by %s", cfg.Port, "9090", configFileEnvName)
	}
}

// TestLoadRejectsAMissingExplicitConfigFile verifies that a path the operator
// gave is not silently ignored: a typo there would otherwise start an instance
// that drops every setting the file holds.
//
// [Ja] TestLoadRejectsAMissingExplicitConfigFile は、運用者が指定したパスが黙って無視され
// ないことを検証する。そうでなければ、パスの打ち間違いがファイルの設定をすべて捨てた
// インスタンスを起動させてしまう。
func TestLoadRejectsAMissingExplicitConfigFile(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv(configFileEnvName, filepath.Join(t.TempDir(), "absent.toml"))

	if _, err := Load(); err == nil {
		t.Fatalf("Load() should fail when the file named by %s is not there", configFileEnvName)
	}
}

// TestLoadRejectsUnknownConfigFileKeys verifies that a misspelled setting stops
// startup and is named, rather than leaving an instance that runs as if the
// setting had never been written.
//
// [Ja] TestLoadRejectsUnknownConfigFileKeys は、綴りを誤った設定が起動を止め、その名前が
// 示されることを検証する。設定を書いていないのと同じ状態で動くインスタンスを残さないため。
func TestLoadRejectsUnknownConfigFileKeys(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, "[email.smtp]\nhostname = \"smtp.example.dev\"\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should reject a key that matches no setting")
	}
	if !strings.Contains(err.Error(), "email.smtp.hostname") {
		t.Errorf("the error should name the unknown key, got: %v", err)
	}
}

// TestLoadConfigFileSyntaxErrorOmitsTheFileContents verifies that a syntax error
// is reported by position without quoting what the file says there. The parser's
// own message quotes the token it stumbled on, and that token can be a password.
//
// [Ja] TestLoadConfigFileSyntaxErrorOmitsTheFileContents は、構文エラーが位置で報告され、
// ファイルの記述内容を引用しないことを検証する。パーサー自身のメッセージはつまずいた
// トークンを引用し、そのトークンはパスワードでありうる。
func TestLoadConfigFileSyntaxErrorOmitsTheFileContents(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, "[email.smtp]\npassword = hunter2\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should reject a configuration file it cannot parse")
	}
	if strings.Contains(err.Error(), "hunter") {
		t.Errorf("the error should not quote the file, got: %v", err)
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("the error should name the position of the problem, got: %v", err)
	}
}

// TestLoadMissingSettingNamesBothSources verifies that a required setting missing
// from both sources is reported with both of its names, so that an operator who
// configures either one can act on the message.
//
// [Ja] TestLoadMissingSettingNamesBothSources は、どちらの入力にも無い必須の設定が両方の
// 名前とともに報告されることを検証する。どちらで設定している運用者もメッセージから対処
// できるようにするため。
func TestLoadMissingSettingNamesBothSources(t *testing.T) {
	clearEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should fail when no source configures the port")
	}
	if !strings.Contains(err.Error(), "GROOBB_PORT") || !strings.Contains(err.Error(), "server.port") {
		t.Errorf("the error should name both sources of the setting, got: %v", err)
	}
}

// TestLoadInvalidSettingNamesTheSourceItCameFrom verifies that a bad value from
// the file is reported against the file. Naming the environment variable instead
// would send an operator looking where they never set anything.
//
// [Ja] TestLoadInvalidSettingNamesTheSourceItCameFrom は、ファイル由来の不正な値がファイル
// に対して報告されることを検証する。代わりに環境変数を名指しすると、運用者は何も設定して
// いない場所を探すことになる。
func TestLoadInvalidSettingNamesTheSourceItCameFrom(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(fullConfigFile, "port = 587", "port = 70000", 1))

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should reject an SMTP port outside the valid range")
	}
	if !strings.Contains(err.Error(), "email.smtp.port") {
		t.Errorf("the error should name the file key the value came from, got: %v", err)
	}
	if strings.Contains(err.Error(), "GROOBB_SMTP_PORT") {
		t.Errorf("the error should not name the environment variable, which sets nothing here, got: %v", err)
	}
}

// TestLoadReadsTheTurnstileDisableFlagFromTheConfigFile verifies that a boolean
// setting works from the file, which is the only place it is written as a
// boolean rather than as text.
//
// [Ja] TestLoadReadsTheTurnstileDisableFlagFromTheConfigFile は、真偽値の設定がファイル
// から機能することを検証する。真偽値として書かれるのはファイルだけであるため。
func TestLoadReadsTheTurnstileDisableFlagFromTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(
		strings.Replace(fullConfigFile, `env = "prod"`, `env = "test"`, 1),
		"disable = false", "disable = true", 1))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.TurnstileSiteKey != "" || cfg.TurnstileSecretKey != "" {
		t.Errorf("the keys should be cleared, got site=%q secret=%q", cfg.TurnstileSiteKey, cfg.TurnstileSecretKey)
	}
}

// TestExampleConfigFileLoads verifies that the sample file distributed with the
// server is a working configuration. Copying it is the starting point of a
// self-hosted setup, and the strict decoding means a key that drifts from the
// schema would stop the instance of whoever copied it.
//
// [Ja] TestExampleConfigFileLoads は、サーバーに同梱するサンプルファイルが実際に動く設定で
// あることを検証する。セルフホストの設定はこのファイルのコピーから始まり、デコードは厳密で
// あるため、スキーマからずれたキーはコピーした人のインスタンスを止めてしまう。
func TestExampleConfigFileLoads(t *testing.T) {
	// The path is resolved before clearEnv moves the test out of the package
	// directory.
	//
	// [Ja] パスは、clearEnv がテストをパッケージのディレクトリから移す前に解決する。
	path, err := filepath.Abs(filepath.Join("..", "..", "groobb.example.toml"))
	if err != nil {
		t.Fatalf("failed to resolve the path of the example configuration file: %v", err)
	}

	clearEnv(t)
	// The example deliberately leaves the continuation token key and the SMTP
	// host empty so that a copied file cannot start with a public key or on a
	// placeholder relay. Supply test-only values for both while checking the rest
	// of the example through Load.
	//
	// [Ja] サンプルは、コピーしたファイルが公開済みの鍵やプレースホルダーのリレーで起動
	// しないよう、continuation token の鍵と SMTP のホストを意図的に空にしている。残りの
	// サンプルを Load で確認する間だけ、両方にテスト用の値を与える。
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "groobb-test-continuation-token-key-32-bytes")
	t.Setenv("GROOBB_SMTP_HOST", "smtp.example.dev")
	t.Setenv(configFileEnvName, path)

	if _, err := Load(); err != nil {
		t.Fatalf("the example configuration file should load: %v", err)
	}
}

// TestLoadConfigFileTypeErrorOmitsTheValue verifies that a decoding error the
// parser reports by type is passed through without the value, and that it names
// the file. configFileError keeps the message of these errors on the grounds
// that they report types rather than values, and that ground is what this pins:
// a library that started quoting the value would put a secret into the startup
// log.
//
// [Ja] TestLoadConfigFileTypeErrorOmitsTheValue は、パーサーが型として報告するデコード
// エラーが値を伴わずに通され、かつファイル名を挙げることを検証する。configFileError が
// この種のエラーのメッセージを残しているのは、それが値ではなく型を報告するからであり、
// 本テストはその前提を固定する。値を引用するようになったライブラリは、秘密情報を起動時の
// ログへ載せることになる。
func TestLoadConfigFileTypeErrorOmitsTheValue(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, "[security]\ncontinuation_token_key = 1234567890\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should reject a value whose type does not match the setting")
	}
	if strings.Contains(err.Error(), "1234567890") {
		t.Errorf("the error should not quote the file, got: %v", err)
	}
	if !strings.Contains(err.Error(), defaultConfigFileName) {
		t.Errorf("the error should name the file it failed on, got: %v", err)
	}
}

// TestLoadRejectsPortZeroFromTheConfigFile verifies that a port written as 0 is
// reported as the out-of-range value it is. The file carries numbers as numbers,
// so 0 would be the zero value of the field; reporting it as a setting nobody
// configured would tell an operator who wrote it that they did not, and port 0
// reaches the kernel as "any free port".
//
// [Ja] TestLoadRejectsPortZeroFromTheConfigFile は、0 と書かれたポートが範囲外の値として
// 報告されることを検証する。ファイルは数値を数値として運ぶため 0 はフィールドのゼロ値に
// あたるが、これを「誰も設定していない」と報告すると、書いた運用者に書いていないと伝える
// ことになる。ポート 0 はカーネルには「空いている任意のポート」として届く。
func TestLoadRejectsPortZeroFromTheConfigFile(t *testing.T) {
	tests := []struct {
		name    string
		old     string
		fileKey string
	}{
		{name: "server", old: "port = 9090", fileKey: "server.port"},
		{name: "SMTP", old: "port = 587", fileKey: "email.smtp.port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			writeConfigFile(t, strings.Replace(fullConfigFile, tt.old, "port = 0", 1))

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() should reject %s = 0", tt.fileKey)
			}
			if !strings.Contains(err.Error(), "between 1 and 65535") {
				t.Errorf("the error should report the value as out of range, got: %v", err)
			}
			if !strings.Contains(err.Error(), tt.fileKey) {
				t.Errorf("the error should name the file key the value came from, got: %v", err)
			}
		})
	}
}
