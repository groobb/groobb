package config

import (
	"fmt"
	"os"
	"strconv"
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
