package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// setting is one resolved configuration value together with where it came
// from. The origin is kept so that a message about the value names the source
// the operator actually used: reporting a bad value from the file as a problem
// with an environment variable would send them looking in the wrong place.
//
// [Ja] setting は解決済みの設定値 1 つと、その値がどこから来たかを保持します。由来を
// 保つのは、値についてのメッセージが運用者の実際に使った入力を指すようにするためです。
// ファイルの誤った値を環境変数の問題として報告すると、運用者は誤った場所を探すことに
// なります。
type setting struct {
	envName string
	fileKey string
	value   string
	fromEnv bool
}

// newSetting resolves a setting: the environment variable wins when it holds a
// non-empty value, and the configuration file provides the value otherwise.
//
// An environment variable set to an empty string counts as unset. Not every way
// of launching a process can tell the two apart, and treating "" as an override
// would let a stray `GROOBB_EMAIL_FROM=` blank out the address the file
// configures instead of leaving it alone.
//
// [Ja] newSetting は設定を解決します。環境変数が空でない値を持つ場合はそれを採用し、
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

// isSet reports whether either source provided a value.
//
// [Ja] isSet はどちらかの入力が値を与えたかどうかを返します。
func (s setting) isSet() bool {
	return s.value != ""
}

// names renders both names of the setting, for a message about the setting
// itself rather than about the value one source gave it.
//
// [Ja] names は設定の両方の名前を整形します。片方の入力が与えた値についてではなく、
// 設定そのものについてのメッセージで使います。
func (s setting) names() string {
	return fmt.Sprintf("%s (%q in the configuration file)", s.envName, s.fileKey)
}

// source names where the value came from.
//
// [Ja] source は値がどこから来たかを示します。
func (s setting) source() string {
	if s.fromEnv {
		return fmt.Sprintf("the environment variable %s", s.envName)
	}

	return fmt.Sprintf("%q in the configuration file", s.fileKey)
}

// missingError reports that a required setting was given by neither source.
//
// [Ja] missingError は、必須の設定がどちらの入力からも与えられなかったことを報告します。
func (s setting) missingError() error {
	return fmt.Errorf("%s is required, but is not configured", s.names())
}

// missingWhenError is missingError for a setting that becomes required under a
// condition, so that the message says what made it required.
//
// [Ja] missingWhenError は、ある条件のもとで必須になる設定向けの missingError です。
// 何によって必須になったのかをメッセージに含めます。
func (s setting) missingWhenError(condition string) error {
	return fmt.Errorf("%s is required when %s, but is not configured", s.names(), condition)
}

// tcpPort parses the setting as a TCP port. subject names the port in the
// message, because a configuration holds more than one of them and the operator
// needs to know which one they got wrong.
//
// The range is checked here rather than left to the listener or the relay, so
// that a wrong value stops startup naming the setting instead of surfacing
// later as a failure to bind or as mail that never leaves. Port 0 is rejected
// with the rest: the kernel reads it as "any free port", which would leave an
// instance listening somewhere nobody can predict.
//
// [Ja] tcpPort は設定を TCP ポートとして解析します。subject をメッセージに含めるのは、
// 設定が複数のポートを持つため、運用者がどれを誤ったのかを知る必要があるからです。
//
// 範囲の検査をリスナーやリレーに委ねずここで行うのは、誤った値が、後になって bind の
// 失敗や「届かないメール」として現れるのではなく、設定名を挙げて起動を止めるように
// するためです。ポート 0 も併せて拒否します。カーネルはこれを「空いている任意のポート」
// と解釈するため、誰にも予測できない場所で待ち受けるインスタンスが生まれます。
func (s setting) tcpPort(subject string) (int, error) {
	port, err := strconv.Atoi(s.value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("the %s from %s must be a TCP port between 1 and 65535, but is %q", subject, s.source(), s.value)
	}

	return port, nil
}

// httpBaseURL validates an optional public base URL. Leaving the setting empty
// is valid, but a supplied value must be safe to concatenate with an
// application path: it names an HTTP(S) address by scheme and host alone, and
// carries nothing after the host that would absorb or reinterpret the path.
//
// A path is rejected along with the rest, a bare trailing slash included. Every
// address this application builds is rooted at "/", so a base URL carrying a
// path would name pages under a prefix the instance does not serve, and the
// absolute URLs built from it would point at addresses that answer 404.
//
// [Ja] httpBaseURL は任意の公開ベース URL を検証します。未設定は有効ですが、指定する
// 値はアプリケーションのパスと安全に連結できる必要があります。スキームとホストだけで
// HTTP(S) のアドレスを名指し、パスを吸収・再解釈するものをホストの後ろに持たないことを
// 検証します。
//
// パスは、末尾スラッシュだけのものも含めて拒否します。このアプリケーションが組み立てる
// アドレスはすべて "/" を根に持つため、パスを持つベース URL は、インスタンスが配信しない
// 接頭辞の下のページを名指すことになり、そこから組み立てた絶対 URL は 404 を返すアドレスを
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

// intFileValue renders a numeric setting from the file as text, so that it
// resolves and is validated through the same path as its environment variable,
// which is always text. A nil value is a key the file leaves out; a written
// value is rendered as it stands, including a zero, so that validation reports
// it as the out-of-range value it is rather than as a setting nobody wrote.
//
// [Ja] intFileValue はファイルの数値設定をテキストとして表現します。常にテキストである
// 環境変数と同じ経路で解決・検証するためです。nil はファイルが書いていないキーを表し、
// 書かれた値は 0 も含めてそのまま表現します。0 を検証が「誰も書いていない設定」ではなく
// 範囲外の値として報告できるようにするためです。
func intFileValue(value *int) string {
	if value == nil {
		return ""
	}

	return strconv.Itoa(*value)
}

// boolFileValue renders a boolean setting from the file as text, for the same
// reason as intFileValue. false and absent are the same thing for a flag that
// only switches something on.
//
// [Ja] boolFileValue はファイルの真偽値設定をテキストとして表現します。理由は
// intFileValue と同じです。何かを有効にするだけのフラグにとって、false と未記載は
// 同じことです。
func boolFileValue(value bool) string {
	if !value {
		return ""
	}

	return "true"
}

// listFileValue renders a list setting from the file as the comma-separated
// text its environment variable carries, for the same reason as intFileValue.
// An empty list is the same as an absent key: both leave the setting off.
//
// Joining loses nothing for the values a list setting takes here (addresses and
// CIDR blocks, which hold no comma). An entry that did hold one is split back
// into pieces, and validation reports them as the entries they became rather
// than as the one that was written.
//
// [Ja] listFileValue はファイルのリスト設定を、対応する環境変数が運ぶカンマ区切りの
// テキストとして表現します。理由は intFileValue と同じです。空のリストはキーの不在と
// 同じことで、どちらも設定を無効のままにします。
//
// ここでリスト設定が取る値 (カンマを含まないアドレスと CIDR ブロック) にとって、連結で
// 失われるものはありません。カンマを含む項目は分割し直され、検証はそれを書かれたままの
// 項目としてではなく、分割後の項目として報告します。
func listFileValue(values []string) string {
	return strings.Join(values, ",")
}
