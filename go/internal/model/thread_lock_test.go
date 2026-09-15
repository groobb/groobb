package model_test

import (
	"slices"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
)

// TestThread_LockReasonsは、スレッドがどこで投稿を受け付けなくなるかと、そのとき
// どの理由を述べるかを検証します。管理者によるロック・上限の件数を持つこと・その両方が
// 同時に成立することのそれぞれについて、管理者の判断が先に来ることを確かめます。
//
// 件数を書き下さずThreadPostLimitから組み立てるのは、上限を1箇所で変えたときに、テストが
// 確かめる境界もともに動くようにするためです。上限を越えた件数を含めるのは、ロックが
// 信頼できるためにはそこでの答えが正しくなければならないからです。行き過ぎたスレッドは
// 閉じているのであって、番号を踏み外したことで開き直りはしません。
func TestThread_LockReasons(t *testing.T) {
	t.Parallel()

	lockedAt := time.Date(2026, 9, 12, 10, 44, 48, 0, time.UTC)

	tests := []struct {
		name       string
		postsCount int
		lockedAt   *time.Time
		want       []model.ThreadLockReason
	}{
		{
			name:       "最初の投稿だけを持つスレッド",
			postsCount: 1,
			want:       nil,
		},
		{
			name:       "上限まであと1件",
			postsCount: model.ThreadPostLimit - 1,
			want:       nil,
		},
		{
			name:       "管理者がロックした",
			postsCount: 1,
			lockedAt:   &lockedAt,
			want:       []model.ThreadLockReason{model.ThreadLockReasonLockedByModerator},
		},
		{
			name:       "上限ちょうど",
			postsCount: model.ThreadPostLimit,
			want:       []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
		},
		{
			name:       "上限を超えている",
			postsCount: model.ThreadPostLimit + 1,
			want:       []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
		},
		{
			name:       "管理者がロックし、かつ上限に達している",
			postsCount: model.ThreadPostLimit,
			lockedAt:   &lockedAt,
			want: []model.ThreadLockReason{
				model.ThreadLockReasonLockedByModerator,
				model.ThreadLockReasonPostLimitReached,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			thread := &model.Thread{PostsCount: tt.postsCount, LockedAt: tt.lockedAt}
			before := *thread

			got := thread.LockReasons()
			if !slices.Equal(got, tt.want) {
				t.Errorf("Thread{PostsCount: %d, LockedAt: %v}.LockReasons() = %v、期待値 = %v", tt.postsCount, tt.lockedAt, got, tt.want)
			}

			// 問うことはスレッドを見つけたときのままにしておく。これから書き込む行に
			// 対して呼び出し元が問うても、その答えが書き込まれるものの一部にならないため
			// である。
			if *thread != before {
				t.Errorf("LockReasons()を呼んだ後のThread{PostsCount: %d} = %+v、期待値 = %+v", tt.postsCount, *thread, before)
			}
		})
	}
}

// TestThreadLockReasonsは、集合がスレッドをロックしうる理由を保持すること、および
// 受け取ったものを書き換える呼び出し側が、次の呼び出し側の見るものを変えられないことを
// 検証します。この集合はロック中のスレッドの案内を突き合わせる相手であるため、これを
// 書き換えられる呼び出し側は、取り除いた理由を素通りさせた検査を通すこともできてしまいます。
func TestThreadLockReasons(t *testing.T) {
	t.Parallel()

	want := []model.ThreadLockReason{
		model.ThreadLockReasonLockedByModerator,
		model.ThreadLockReasonPostLimitReached,
	}
	if got := model.ThreadLockReasons(); !slices.Equal(got, want) {
		t.Errorf("ThreadLockReasons() = %v、期待値 = %v", got, want)
	}

	model.ThreadLockReasons()[0] = model.ThreadLockReason("edited")
	if got := model.ThreadLockReasons(); !slices.Equal(got, want) {
		t.Errorf("書き換えた後のThreadLockReasons() = %v、期待値 = %v", got, want)
	}
}
