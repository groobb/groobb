package usecase_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestGetThreadModerationUsecase_Execute_Thread verifies that a screen acting on
// the thread itself is handed the thread and no post.
//
// [Ja] TestGetThreadModerationUsecase_Execute_Threadは、スレッド自身に対して働きかける
// 画面が、スレッドを受け取り、投稿は受け取らないことを検証します。
func TestGetThreadModerationUsecase_Execute_Thread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	output, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if output.Thread.ID != f.thread.ID {
		t.Errorf("Thread.ID = %s, want %s", output.Thread.ID, f.thread.ID)
	}
	if output.Post != nil {
		t.Errorf("Post = %+v, want nil", output.Post)
	}
}

// TestGetThreadModerationUsecase_Execute_Post verifies that a screen acting on
// one post is handed that post alongside the thread it stands in, and the
// account that wrote it, which is what the screen names the post by.
//
// [Ja] TestGetThreadModerationUsecase_Execute_Postは、投稿1件に対して働きかける画面が、
// その投稿を、それが立っているスレッドとともに受け取ること、そしてそれを書いたアカウントも
// 受け取ることを検証します。画面が投稿を名指すのにそれを使うためです。
func TestGetThreadModerationUsecase_Execute_Post(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	number := 1

	output, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Number:   &number,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if output.Thread.ID != f.thread.ID {
		t.Errorf("Thread.ID = %s, want %s", output.Thread.ID, f.thread.ID)
	}
	if output.Post == nil {
		t.Fatal("Post = nil, want 非nil")
	}
	if output.Post.Number != number {
		t.Errorf("Post.Number = %d, want %d", output.Post.Number, number)
	}
	if output.PostAuthor == nil {
		t.Fatal("PostAuthor = nil, want 非nil")
	}
	if output.PostAuthor.ID != f.starter {
		t.Errorf("PostAuthor.ID = %s, want %s", output.PostAuthor.ID, f.starter)
	}
}

// TestGetThreadModerationUsecase_Execute_PostWithWithdrawnAuthor verifies that a
// post whose author has withdrawn is still handed to the screen, with no account
// to name. The post stays in its thread either way, so a screen that refused it
// would leave the one post nobody can be asked about.
//
// [Ja] TestGetThreadModerationUsecase_Execute_PostWithWithdrawnAuthorは、作者が退会した
// 投稿も、名指すアカウントが無いまま画面へ渡されることを検証します。どちらの場合も投稿は
// スレッドに残るため、それを拒む画面は、誰にも問い合わせられない投稿を1件残すことになります。
func TestGetThreadModerationUsecase_Execute_PostWithWithdrawnAuthor(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	number := 1
	if err := repository.NewUserRepository(f.db).SoftDeleteAndAnonymize(ctx, f.starter, "deleted@example.com", "deleted1"); err != nil {
		t.Fatalf("テスト用利用者の退会に失敗: %v", err)
	}

	output, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
		Number:   &number,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if output.Post == nil {
		t.Fatal("Post = nil, want 非nil")
	}
	if output.PostAuthor != nil {
		t.Errorf("PostAuthor = %+v, want nil", output.PostAuthor)
	}
}

// TestGetThreadModerationUsecase_Execute_ScopedActor verifies that any one of
// the operations on a thread admits the screens that lead to them, and that a
// role holding none of them is refused.
//
// The screen is one step before the operation, so it is not reserved to the
// scope of the operation a particular page carries out: a visitor who may
// unpublish a post reaches it, and whether they may also lock the thread is
// answered when they submit.
//
// [Ja] TestGetThreadModerationUsecase_Execute_ScopedActorは、スレッドに対する操作の
// どれか1つが、そこへ至る画面を許すこと、そしてそのいずれも持たないロールが拒否されることを
// 検証します。
//
// 画面は操作の1つ手前にあるため、特定のページが行う操作のスコープに留保されてはいません。
// 投稿を非公開にしてよい訪問者はそこへ辿り着き、スレッドをロックしてもよいかどうかは、その人が
// 送信したときに答えられます。
func TestGetThreadModerationUsecase_Execute_ScopedActor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		roleName   model.RoleName
		scopes     []model.Scope
		wantOpened bool
	}{
		{
			name:       "thread_lock:write だけを持つ",
			roleName:   model.RoleName("locker"),
			scopes:     []model.Scope{model.ScopeThreadLockWrite},
			wantOpened: true,
		},
		{
			name:       "thread_unpublication:write だけを持つ",
			roleName:   model.RoleName("thread_hider"),
			scopes:     []model.Scope{model.ScopeThreadUnpublicationWrite},
			wantOpened: true,
		},
		{
			name:       "post_unpublication:write だけを持つ",
			roleName:   model.RoleName("post_hider"),
			scopes:     []model.Scope{model.ScopePostUnpublicationWrite},
			wantOpened: true,
		},
		{
			name:       "利用者の一覧のスコープだけを持つ",
			roleName:   model.RoleName("user_reader"),
			scopes:     []model.Scope{model.ScopeUserRead},
			wantOpened: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, ctx := newThreadModerationUsecases(t)
			actor := seedScopedActor(t, f.db, tt.roleName, tt.scopes)

			_, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
				Actor:    usecase.UserActor(actor),
				ThreadID: f.thread.ID,
			})

			if tt.wantOpened {
				if err != nil {
					t.Fatalf("Execute() error = %v, want nil", err)
				}
				return
			}
			assertAppErrCode(t, err, model.AppErrCodeForbidden)
		})
	}
}

// TestGetThreadModerationUsecase_Execute_WithoutPermission verifies that an
// account holding no role is refused, rather than being shown what a thread
// holds through a screen meant for administrators.
//
// [Ja] TestGetThreadModerationUsecase_Execute_WithoutPermissionは、ロールを1つも
// 持たないアカウントが拒否されることを検証します。管理者のための画面を通じてスレッドの
// 持つものを見せられることはありません。
func TestGetThreadModerationUsecase_Execute_WithoutPermission(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	_, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(testutil.NewUserBuilder(t, f.db).Build()),
		ThreadID: f.thread.ID,
	})

	assertAppErrCode(t, err, model.AppErrCodeForbidden)
}

// TestGetThreadModerationUsecase_Execute_UnknownThread verifies that an address
// naming no thread is answered as a missing resource.
//
// [Ja] TestGetThreadModerationUsecase_Execute_UnknownThreadは、どのスレッドも名指して
// いないアドレスが、リソースの不在として答えられることを検証します。
func TestGetThreadModerationUsecase_Execute_UnknownThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)

	_, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID + 1000,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
}

// TestGetThreadModerationUsecase_Execute_UnpublishedThread verifies that a
// thread the community no longer shows is not offered as the target of a
// further operation.
//
// [Ja] TestGetThreadModerationUsecase_Execute_UnpublishedThreadは、コミュニティが
// もう示していないスレッドが、さらなる操作の対象として差し出されないことを検証します。
func TestGetThreadModerationUsecase_Execute_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newThreadModerationUsecases(t)
	unpublishThread(t, f.db, f.thread.ID)

	_, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(f.admin),
		ThreadID: f.thread.ID,
	})

	assertAppErrCode(t, err, model.AppErrCodeResourceUnpublished)
}

// TestGetThreadModerationUsecase_Execute_MissingPost verifies that a reply
// number the thread never issued and one whose post is already out of view are
// both answered as a missing resource.
//
// The two are told apart nowhere the visitor can see, because a page naming
// either has nothing to put in front of the administrator.
//
// [Ja] TestGetThreadModerationUsecase_Execute_MissingPostは、スレッドが一度も発行して
// いないレス番号と、既に視界の外にある投稿の番号が、どちらもリソースの不在として答えられる
// ことを検証します。
//
// 2つが訪問者に見える場所で区別されることはありません。どちらを名指すページも、管理者の前に
// 置くものを持たないためです。
func TestGetThreadModerationUsecase_Execute_MissingPost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		number int
		hide   bool
	}{
		{name: "発行されていないレス番号", number: 99},
		{name: "既に非公開の投稿", number: 1, hide: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, ctx := newThreadModerationUsecases(t)
			if tt.hide {
				post := findPost(t, f.db, f.thread.ID, tt.number)
				if err := repository.NewPostRepository(f.db).Unpublish(ctx, post.ID); err != nil {
					t.Fatalf("テスト用投稿の非公開に失敗: %v", err)
				}
			}

			number := tt.number
			_, err := f.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
				Actor:    usecase.UserActor(f.admin),
				ThreadID: f.thread.ID,
				Number:   &number,
			})

			assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
		})
	}
}
