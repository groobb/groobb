package model_test

import (
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
)

// TestPostIntervalWaitは、ある時点で1人の投稿の間隔がどれだけになるかを検証します。
// 尽きていれば何も無く、そうでなければ残りを整数秒で返します。時刻は時計から取らずに
// 固定しているため、境界をちょうどの値で確かめられ、どのテストも待ちません。
func TestPostIntervalWait(t *testing.T) {
	t.Parallel()

	lastPostedAt := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		elapsed time.Duration
		want    time.Duration
	}{
		{
			name:    "投稿した瞬間は間隔がまるごと残る",
			elapsed: 0,
			want:    model.PostInterval,
		},
		{
			name:    "1秒経つと残りは9秒",
			elapsed: 1 * time.Second,
			want:    9 * time.Second,
		},
		{
			name:    "端数の残りは切り上げる",
			elapsed: 8500 * time.Millisecond,
			want:    2 * time.Second,
		},
		{
			name:    "1秒に満たない残りも1秒として伝える",
			elapsed: 9800 * time.Millisecond,
			want:    1 * time.Second,
		},
		{
			name:    "ちょうど間隔が経てば待ち時間は無い",
			elapsed: model.PostInterval,
			want:    0,
		},
		{
			name:    "間隔を過ぎていれば待ち時間は無い",
			elapsed: model.PostInterval + time.Millisecond,
			want:    0,
		},
		{
			name:    "ずっと前の投稿でも待ち時間は無い",
			elapsed: 24 * time.Hour,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.PostIntervalWait(lastPostedAt, lastPostedAt.Add(tt.elapsed))
			if got != tt.want {
				t.Errorf("PostIntervalWait(elapsed=%s) = %s、期待値 = %s", tt.elapsed, got, tt.want)
			}
		})
	}
}

// TestParsePostNumberは、ParsePostNumberがレス番号の10進表記を受け付け、どの投稿も
// 名指さないものをすべて拒否することを検証します。アドレスがそもそも投稿を名指しうるかを
// 決めるルートが、クエリを発行せずにそれを行えるようにするためです。
//
// strconvが読み取るがどのスレッドも書かない綴り (先頭のゼロやプラス記号) は、スレッドのidと
// 同じく解析でき、その後にルートが描くものは、パスではなく得られた番号から組み立てられます。
func TestParsePostNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want int
		ok   bool
	}{
		{name: "10進数の番号", raw: "12", want: 12, ok: true},
		{name: "最初の投稿", raw: "1", want: 1, ok: true},
		{name: "先頭にゼロが付く", raw: "012", want: 12, ok: true},
		{name: "プラス記号が付く", raw: "+12", want: 12, ok: true},
		{name: "ゼロ", raw: "0", ok: false},
		{name: "負の数", raw: "-12", ok: false},
		{name: "空文字列", raw: "", ok: false},
		{name: "数値でない", raw: "twelve", ok: false},
		{name: "数字の後ろに文字が続く", raw: "12unpublication", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParsePostNumber(tt.raw)

			if ok != tt.ok {
				t.Fatalf("ParsePostNumber(%q)のok = %v、期待値 = %v", tt.raw, ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got != tt.want {
				t.Errorf("ParsePostNumber(%q) = %d、期待値 = %d", tt.raw, got, tt.want)
			}
		})
	}
}
