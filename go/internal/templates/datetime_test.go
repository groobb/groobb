package templates_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates"
)

// TestRelativeTime verifies that an instant is reported in the largest unit it
// fills, in the locale of the request. The boundaries are what decide which unit
// a reader is shown, so each is asserted from just inside it: a minute short of
// an hour is still minutes, a minute past it is hours.
//
// [Ja] TestRelativeTime は、ある時点がそれを満たす最大の単位で、リクエストのロケールで
// 報告されることを検証します。読み手にどの単位が示されるかを決めるのは境界であるため、
// それぞれを境界のすぐ内側から検証します。1 時間に 1 分足りなければまだ分であり、
// 1 分過ぎれば時間です。
func TestRelativeTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ago    time.Duration
		wantJa string
		wantEn string
	}{
		{name: "1 分未満", ago: 30 * time.Second, wantJa: "たった今", wantEn: "Just now"},
		{name: "未来の時刻", ago: -time.Minute, wantJa: "たった今", wantEn: "Just now"},
		{name: "分単位の下限", ago: time.Minute, wantJa: "1 分前", wantEn: "1 minute ago"},
		{name: "分単位の上限", ago: 59 * time.Minute, wantJa: "59 分前", wantEn: "59 minutes ago"},
		{name: "時間単位の下限", ago: time.Hour, wantJa: "1 時間前", wantEn: "1 hour ago"},
		{name: "時間単位の上限", ago: 23 * time.Hour, wantJa: "23 時間前", wantEn: "23 hours ago"},
		{name: "日単位の下限", ago: 24 * time.Hour, wantJa: "1 日前", wantEn: "1 day ago"},
		{name: "日単位の上限", ago: 29 * 24 * time.Hour, wantJa: "29 日前", wantEn: "29 days ago"},
		{name: "月単位の下限", ago: 30 * 24 * time.Hour, wantJa: "1 か月前", wantEn: "1 month ago"},
		{name: "月単位の上限", ago: 364 * 24 * time.Hour, wantJa: "12 か月前", wantEn: "12 months ago"},
		{name: "年単位の下限", ago: 365 * 24 * time.Hour, wantJa: "1 年前", wantEn: "1 year ago"},
		{name: "複数年", ago: 800 * 24 * time.Hour, wantJa: "2 年前", wantEn: "2 years ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			at := time.Now().Add(-tt.ago)

			if got := templates.RelativeTime(i18n.SetLocale(context.Background(), i18n.LangJa), at); got != tt.wantJa {
				t.Errorf("RelativeTime(ja, -%v) = %q, want %q", tt.ago, got, tt.wantJa)
			}
			if got := templates.RelativeTime(i18n.SetLocale(context.Background(), i18n.LangEn), at); got != tt.wantEn {
				t.Errorf("RelativeTime(en, -%v) = %q, want %q", tt.ago, got, tt.wantEn)
			}
		})
	}
}

// TestMachineDateTime verifies that the instant is rendered in UTC whatever zone
// it arrives in, since the datetime attribute is what carries the exact moment
// while the element's text only says how long ago it was.
//
// [Ja] TestMachineDateTime は、時点がどのタイムゾーンで渡されても UTC で描画される
// ことを検証します。要素のテキストがどれだけ前かを述べるだけであるのに対し、正確な
// 時点を運ぶのが datetime 属性だからです。
func TestMachineDateTime(t *testing.T) {
	t.Parallel()

	tokyo := time.FixedZone("JST", 9*60*60)
	at := time.Date(2026, 8, 26, 23, 30, 0, 123456789, tokyo)

	if got, want := templates.MachineDateTime(at), "2026-08-26T14:30:00.123456789Z"; got != want {
		t.Errorf("MachineDateTime() = %q, want %q", got, want)
	}
	if !strings.HasSuffix(templates.MachineDateTime(time.Now()), "Z") {
		t.Error("MachineDateTime() は UTC を表す Z で終わるはず")
	}
}
