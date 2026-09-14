package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// moderationRepos bundles the moderation log repository with the repositories
// the community's content is built from. An entry names the thread, post or
// account an operation was aimed at, so a test of the log creates those first.
//
// [Ja] moderationReposは、操作履歴のリポジトリを、コミュニティの中身を組み立てる
// リポジトリ群と束ねる。記録は操作の対象となったスレッド・投稿・アカウントを名指すため、
// 履歴のテストはまずそれらを作ることになる。
type moderationRepos struct {
	*contentRepos
	log *repository.ModerationLogRepository
}

// newModerationRepos builds the repositories over a fresh database.
//
// [Ja] newModerationReposは新しいデータベース上にリポジトリ群を作る。
func newModerationRepos(t *testing.T) (*moderationRepos, context.Context) {
	t.Helper()

	content, ctx := newContentRepos(t)
	return &moderationRepos{
		contentRepos: content,
		log:          repository.NewModerationLogRepository(content.db),
	}, ctx
}

// listLogs reads one page big enough to hold everything a test wrote, for the
// assertions whose subject is what a write left behind rather than the paging.
//
// [Ja] listLogsは、テストが書いたものがすべて収まる大きさの1ページを読む。ページ分割
// ではなく、書き込みが何を残したかを主題とする検証のためのものである。
func (r *moderationRepos) listLogs(t *testing.T, ctx context.Context) []*model.ModerationLog {
	t.Helper()

	logs, err := r.log.ListPage(ctx, 100, 0)
	if err != nil {
		t.Fatalf("ListPage() error = %v", err)
	}
	return logs
}

func TestModerationLogRepository_Create(t *testing.T) {
	t.Parallel()

	t.Run("操作者と対象と理由を持つ記録を書いて読み戻せる", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newModerationRepos(t)
		administrator := testutil.NewUserBuilder(t, repos.db).Build()
		target := testutil.NewUserBuilder(t, repos.db).Build()

		created, err := repos.log.Create(ctx, repository.CreateModerationLogInput{
			UserID:       &administrator,
			Action:       model.ModerationActionUserSuspend,
			TargetUserID: &target,
			Reason:       "宣伝の投稿を繰り返したため",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if created.ID == 0 {
			t.Error("Create() log.ID は DB 採番で空でないはず")
		}
		if created.CreatedAt.IsZero() {
			t.Error("log.CreatedAt は DB 既定値で設定されるはず")
		}
		if created.UpdatedAt.IsZero() {
			t.Error("log.UpdatedAt は DB 既定値で設定されるはず")
		}

		logs := repos.listLogs(t, ctx)
		if len(logs) != 1 {
			t.Fatalf("len(ListPage()) = %d, want 1", len(logs))
		}

		got := logs[0]
		if got.ID != created.ID {
			t.Errorf("log.ID = %v, want %v", got.ID, created.ID)
		}
		if got.UserID == nil || *got.UserID != administrator {
			t.Errorf("log.UserID = %v, want %v", got.UserID, administrator)
		}
		if got.Action != model.ModerationActionUserSuspend {
			t.Errorf("log.Action = %q, want %q", got.Action, model.ModerationActionUserSuspend)
		}
		if got.TargetUserID == nil || *got.TargetUserID != target {
			t.Errorf("log.TargetUserID = %v, want %v", got.TargetUserID, target)
		}
		if got.Reason != "宣伝の投稿を繰り返したため" {
			t.Errorf("log.Reason = %q, want %q", got.Reason, "宣伝の投稿を繰り返したため")
		}
		if got.ThreadID != nil {
			t.Errorf("log.ThreadID = %v, want nil (停止はスレッドを名指さない)", got.ThreadID)
		}
		if got.PostID != nil {
			t.Errorf("log.PostID = %v, want nil (停止は投稿を名指さない)", got.PostID)
		}
	})

	// An operation performed outside any account has no row to point at, and a
	// reason is optional, so the entry an operator leaves is one whose operator
	// column is NULL and whose reason is the empty string. Reading it back is
	// what shows those come back as the absence they were written as, rather
	// than as a zero id.
	//
	// [Ja] どのアカウントでもなく行われた操作は指す行を持たず、理由は任意であるため、
	// 運用者が残す記録は、操作者の列がNULLで理由が空文字列のものになる。それを読み戻す
	// ことが、それらが0のidではなく、書かれたとおりの不在として返ることを示す。
	t.Run("操作者と理由が無い記録はNULLと空文字列のまま往復する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newModerationRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "運用者がロックするスレッド")

		if _, err := repos.log.Create(ctx, repository.CreateModerationLogInput{
			Action:   model.ModerationActionThreadLock,
			ThreadID: &thread.ID,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		logs := repos.listLogs(t, ctx)
		if len(logs) != 1 {
			t.Fatalf("len(ListPage()) = %d, want 1", len(logs))
		}

		got := logs[0]
		if got.UserID != nil {
			t.Errorf("log.UserID = %v, want nil (運用者の操作は操作者の行を持たない)", got.UserID)
		}
		if got.Reason != "" {
			t.Errorf("log.Reason = %q, want %q", got.Reason, "")
		}
		if got.ThreadID == nil || *got.ThreadID != thread.ID {
			t.Errorf("log.ThreadID = %v, want %v", got.ThreadID, thread.ID)
		}
		if got.TargetUserID != nil {
			t.Errorf("log.TargetUserID = %v, want nil", got.TargetUserID)
		}
	})

	t.Run("投稿の非公開はスレッドと投稿の両方を名指す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newModerationRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "投稿が非公開にされるスレッド")
		post := repos.createPost(t, ctx, thread.ID, 1, "非公開にされる投稿")

		if _, err := repos.log.Create(ctx, repository.CreateModerationLogInput{
			Action:   model.ModerationActionPostUnpublish,
			ThreadID: &thread.ID,
			PostID:   &post.ID,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		logs := repos.listLogs(t, ctx)
		if len(logs) != 1 {
			t.Fatalf("len(ListPage()) = %d, want 1", len(logs))
		}

		got := logs[0]
		if got.ThreadID == nil || *got.ThreadID != thread.ID {
			t.Errorf("log.ThreadID = %v, want %v", got.ThreadID, thread.ID)
		}
		if got.PostID == nil || *got.PostID != post.ID {
			t.Errorf("log.PostID = %v, want %v", got.PostID, post.ID)
		}
	})
}

// TestModerationLogRepository_Create_RejectsAnActionOutsideTheSet verifies the
// check the repository applies in place of the CHECK the column does not carry:
// an action the application does not perform is refused, and the history is left
// without a row nobody can read.
//
// [Ja] TestModerationLogRepository_Create_RejectsAnActionOutsideTheSetは、列が持たない
// CHECKの代わりにリポジトリが適用する検査を検証する。アプリケーションが行わない操作は
// 拒否され、履歴には誰にも読めない行が残らない。
func TestModerationLogRepository_Create_RejectsAnActionOutsideTheSet(t *testing.T) {
	t.Parallel()

	repos, ctx := newModerationRepos(t)

	tests := []struct {
		name   string
		action model.ModerationAction
	}{
		{name: "アプリケーションが行わない操作", action: "thread_delete"},
		{name: "未設定", action: ""},
		{name: "大文字の表記", action: "THREAD_LOCK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := repos.log.Create(ctx, repository.CreateModerationLogInput{Action: tt.action})
			if err == nil {
				t.Fatalf("Create() error = nil, want an error (action=%q)", tt.action)
			}
			if log != nil {
				t.Errorf("Create() = %v, want nil", log)
			}
		})
	}

	count, err := repos.log.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}
}

// TestModerationLogRepository_ListPage verifies the page the admin screen reads:
// the most recent operation first, and one page at a time.
//
// [Ja] TestModerationLogRepository_ListPageは管理画面が読むページを検証する。新しい操作を
// 先に置き、一度に1ページ分を返すことである。
func TestModerationLogRepository_ListPage(t *testing.T) {
	t.Parallel()

	t.Run("新しい操作から順に、1ページ分を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newModerationRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "ロックと解除を受けるスレッド")

		first := testutil.NewModerationLogBuilder(t, repos.db).
			WithAction(model.ModerationActionThreadLock).
			WithThreadID(thread.ID).
			Build()
		second := testutil.NewModerationLogBuilder(t, repos.db).
			WithAction(model.ModerationActionThreadUnlock).
			WithThreadID(thread.ID).
			Build()
		third := testutil.NewModerationLogBuilder(t, repos.db).
			WithAction(model.ModerationActionThreadUnpublish).
			WithThreadID(thread.ID).
			Build()

		page := func(t *testing.T, limit, offset int) []*model.ModerationLog {
			t.Helper()

			logs, err := repos.log.ListPage(ctx, limit, offset)
			if err != nil {
				t.Fatalf("ListPage(%d, %d) error = %v", limit, offset, err)
			}
			return logs
		}

		firstPage := page(t, 2, 0)
		if len(firstPage) != 2 {
			t.Fatalf("len(ListPage(2, 0)) = %d, want 2", len(firstPage))
		}
		if firstPage[0].ID != third {
			t.Errorf("ListPage(2, 0)[0].ID = %v, want %v (最新の操作)", firstPage[0].ID, third)
		}
		if firstPage[1].ID != second {
			t.Errorf("ListPage(2, 0)[1].ID = %v, want %v", firstPage[1].ID, second)
		}

		secondPage := page(t, 2, 2)
		if len(secondPage) != 1 {
			t.Fatalf("len(ListPage(2, 2)) = %d, want 1", len(secondPage))
		}
		if secondPage[0].ID != first {
			t.Errorf("ListPage(2, 2)[0].ID = %v, want %v", secondPage[0].ID, first)
		}

		beyond := page(t, 2, 4)
		if len(beyond) != 0 {
			t.Errorf("len(ListPage(2, 4)) = %d, want 0 (最後を越えたページは空)", len(beyond))
		}
	})

	t.Run("何も記録が無ければ空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newModerationRepos(t)

		logs, err := repos.log.ListPage(ctx, 50, 0)
		if err != nil {
			t.Fatalf("ListPage() error = %v", err)
		}
		if len(logs) != 0 {
			t.Errorf("len(ListPage()) = %d, want 0", len(logs))
		}
	})
}

func TestModerationLogRepository_Count(t *testing.T) {
	t.Parallel()

	repos, ctx := newModerationRepos(t)
	target := testutil.NewUserBuilder(t, repos.db).Build()

	count, err := repos.log.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("Count() = %d, want 0", count)
	}

	testutil.NewModerationLogBuilder(t, repos.db).
		WithAction(model.ModerationActionUserSuspend).
		WithTargetUserID(target).
		Build()
	testutil.NewModerationLogBuilder(t, repos.db).
		WithAction(model.ModerationActionUserUnsuspend).
		WithTargetUserID(target).
		Build()

	count, err = repos.log.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}
}
