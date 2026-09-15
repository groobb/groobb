package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// settingは解決済みの設定値1つと、その値がどこから来たかを保持します。由来を
// 保つのは、値についてのメッセージが運用者の実際に使った入力を指すようにするためです。
// ファイルの誤った値を環境変数の問題として報告すると、運用者は誤った場所を探すことに
// なります。
type setting struct {
	envName string
	fileKey string
	value   string
	fromEnv bool
}

// newSettingは設定を解決します。環境変数が空でない値を持つ場合はそれを採用し、
// それ以外の場合は設定ファイルの値を使います。
//
// 空文字列が設定された環境変数は未設定として扱います。プロセスの起動方法によっては
// 両者を区別できず、"" を上書きとして扱うと、紛れ込んだ `GROOBB_EMAIL_FROM=` が、
// ファイルの設定するアドレスをそのままにせず空にしてしまうためです。
func newSetting(envName, fileKey, fileValue string) setting {
	if value := os.Getenv(envName); value != "" {
		return setting{envName: envName, fileKey: fileKey, value: value, fromEnv: true}
	}

	return setting{envName: envName, fileKey: fileKey, value: fileValue}
}

// isSetはどちらかの入力が値を与えたかどうかを返します。
func (s setting) isSet() bool {
	return s.value != ""
}

// namesは設定の両方の名前を整形します。片方の入力が与えた値についてではなく、
// 設定そのものについてのメッセージで使います。
func (s setting) names() string {
	return fmt.Sprintf("%s (%q in the configuration file)", s.envName, s.fileKey)
}

// sourceは値がどこから来たかを示します。
func (s setting) source() string {
	if s.fromEnv {
		return fmt.Sprintf("the environment variable %s", s.envName)
	}

	return fmt.Sprintf("%q in the configuration file", s.fileKey)
}

// missingErrorは、必須の設定がどちらの入力からも与えられなかったことを報告します。
func (s setting) missingError() error {
	return fmt.Errorf("%s is required, but is not configured", s.names())
}

// missingWhenErrorは、ある条件のもとで必須になる設定向けのmissingErrorです。
// 何によって必須になったのかをメッセージに含めます。
func (s setting) missingWhenError(condition string) error {
	return fmt.Errorf("%s is required when %s, but is not configured", s.names(), condition)
}

// tcpPortは設定をTCPポートとして解析します。subjectをメッセージに含めるのは、
// 設定が複数のポートを持つため、運用者がどれを誤ったのかを知る必要があるからです。
//
// 範囲の検査をリスナーやリレーに委ねずここで行うのは、誤った値が、後になってbindの
// 失敗や「届かないメール」として現れるのではなく、設定名を挙げて起動を止めるように
// するためです。ポート0も併せて拒否します。カーネルはこれを「空いている任意のポート」
// と解釈するため、誰にも予測できない場所で待ち受けるインスタンスが生まれます。
func (s setting) tcpPort(subject string) (int, error) {
	port, err := strconv.Atoi(s.value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("the %s from %s must be a TCP port between 1 and 65535, but is %q", subject, s.source(), s.value)
	}

	return port, nil
}

// httpBaseURLは任意の公開ベースURLを検証します。未設定は有効ですが、指定する
// 値はアプリケーションのパスと安全に連結できる必要があります。スキームとホストだけで
// HTTP(S) のアドレスを名指し、パスを吸収・再解釈するものをホストの後ろに持たないことを
// 検証します。
//
// パスは、末尾スラッシュだけのものも含めて拒否します。このアプリケーションが組み立てる
// アドレスはすべて "/" を根に持つため、パスを持つベースURLは、インスタンスが配信しない
// 接頭辞の下のページを名指すことになり、そこから組み立てた絶対URLは404を返すアドレスを
// 指します。
func (s setting) httpBaseURL(subject string) (string, error) {
	if !s.isSet() {
		return "", nil
	}

	invalid := func() error {
		return fmt.Errorf("the %s from %s must be an absolute HTTP or HTTPS URL naming only a scheme and a host, without a path (a bare trailing slash included), user information, a query, or a fragment, but is %q", subject, s.source(), s.value)
	}

	parsed, err := url.Parse(s.value)
	if err != nil {
		return "", invalid()
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" ||
		parsed.Path != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery ||
		strings.Contains(s.value, "#") {
		return "", invalid()
	}

	return s.value, nil
}

// intFileValueはファイルの数値設定をテキストとして表現します。常にテキストである
// 環境変数と同じ経路で解決・検証するためです。nilはファイルが書いていないキーを表し、
// 書かれた値は0も含めてそのまま表現します。0を検証が「誰も書いていない設定」ではなく
// 範囲外の値として報告できるようにするためです。
func intFileValue(value *int) string {
	if value == nil {
		return ""
	}

	return strconv.Itoa(*value)
}

// boolFileValueはファイルの真偽値設定をテキストとして表現します。理由は
// intFileValueと同じです。何かを有効にするだけのフラグにとって、falseと未記載は
// 同じことです。
func boolFileValue(value bool) string {
	if !value {
		return ""
	}

	return "true"
}

// listFileValueはファイルのリスト設定を、対応する環境変数が運ぶカンマ区切りの
// テキストとして表現します。理由はintFileValueと同じです。空のリストはキーの不在と
// 同じことで、どちらも設定を無効のままにします。
//
// ここでリスト設定が取る値 (カンマを含まないアドレスとCIDRブロック) にとって、連結で
// 失われるものはありません。カンマを含む項目は分割し直され、検証はそれを書かれたままの
// 項目としてではなく、分割後の項目として報告します。
func listFileValue(values []string) string {
	return strings.Join(values, ",")
}
