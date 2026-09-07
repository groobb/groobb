package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestThread_LockReasons verifies where a thread stops taking posts: it takes
// one until it holds the cap, and from the cap onwards it says the cap is why it
// takes no more.
//
// The counts are written against ThreadPostLimit rather than spelled out, so
// that a cap changed in one place moves the boundary the test checks with it. A
// count above the cap is included because it is what the answer has to be right
// about for the lock to be trustworthy: a thread that overshot is closed, not
// reopened by having missed the number.
//
// [Ja] TestThread_LockReasons は、スレッドがどこで投稿を受け付けなくなるかを検証します。
// 上限の件数を持つまでは受け付け、上限からは、それ以上受け付けない理由が上限であることを
// 述べます。
//
// 件数を書き下さず ThreadPostLimit から組み立てるのは、上限を 1 箇所で変えたときに、
// テストが確かめる境界もともに動くようにするためです。上限を越えた件数を含めるのは、
// ロックが信頼できるためにはそこでの答えが正しくなければならないからです。行き過ぎた
// スレッドは閉じているのであって、番号を踏み外したことで開き直りはしません。
func TestThread_LockReasons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		postsCount int
		want       []model.ThreadLockReason
	}{
		{
			name:       "a thread that has only its first post",
			postsCount: 1,
			want:       nil,
		},
		{
			name:       "one post short of the cap",
			postsCount: model.ThreadPostLimit - 1,
			want:       nil,
		},
		{
			name:       "the cap",
			postsCount: model.ThreadPostLimit,
			want:       []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
		},
		{
			name:       "past the cap",
			postsCount: model.ThreadPostLimit + 1,
			want:       []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			thread := &model.Thread{PostsCount: tt.postsCount}
			before := *thread

			got := thread.LockReasons()
			if !slices.Equal(got, tt.want) {
				t.Errorf("Thread{PostsCount: %d}.LockReasons() = %v, want %v", tt.postsCount, got, tt.want)
			}

			// Asking the question leaves the thread as it was found, so a caller may
			// ask it of a row it is about to write without the answer becoming part
			// of what gets written.
			//
			// [Ja] 問うことはスレッドを見つけたときのままにしておく。これから書き込む行に
			// 対して呼び出し元が問うても、その答えが書き込まれるものの一部にならないため
			// である。
			if *thread != before {
				t.Errorf("Thread{PostsCount: %d} after LockReasons() = %+v, want %+v", tt.postsCount, *thread, before)
			}
		})
	}
}

// TestThreadLockReasons verifies that the set holds the reasons a thread can be
// locked for, and that a caller editing what it receives cannot change what the
// next caller sees. The set is what the notice about a locked thread is checked
// against, so a caller that could edit it could also make that check pass over a
// reason it had removed.
//
// [Ja] TestThreadLockReasons は、集合がスレッドをロックしうる理由を保持すること、および
// 受け取ったものを書き換える呼び出し側が、次の呼び出し側の見るものを変えられないことを
// 検証します。この集合はロック中のスレッドの案内を突き合わせる相手であるため、これを
// 書き換えられる呼び出し側は、取り除いた理由を素通りさせた検査を通すこともできてしまいます。
func TestThreadLockReasons(t *testing.T) {
	t.Parallel()

	want := []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}
	if got := model.ThreadLockReasons(); !slices.Equal(got, want) {
		t.Errorf("ThreadLockReasons() = %v, want %v", got, want)
	}

	model.ThreadLockReasons()[0] = model.ThreadLockReason("edited")
	if got := model.ThreadLockReasons(); !slices.Equal(got, want) {
		t.Errorf("ThreadLockReasons() after an edit = %v, want %v", got, want)
	}
}
