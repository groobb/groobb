package model_test

import (
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
)

// TestPostIntervalWait verifies what the interval between one person's posts
// amounts to at a given moment: nothing once it has run out, and otherwise the
// remainder in whole seconds. The times are fixed rather than taken from the
// clock, so the boundary is checked exactly and no test waits for one.
//
// [Ja] TestPostIntervalWait は、ある時点で 1 人の投稿の間隔がどれだけになるかを検証します。
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
				t.Errorf("PostIntervalWait(elapsed=%s) = %s, want %s", tt.elapsed, got, tt.want)
			}
		})
	}
}
