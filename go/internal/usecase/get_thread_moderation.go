package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetThreadModerationInputはExecuteの入力です。Actorは画面を開いた側、ThreadIDは
// その画面が対象とする/t/{id}、Numberは画面がスレッド自身ではなくその投稿1件を対象とする
// ときのレス番号です。
//
// Numberをポインタとするのは、スレッドに対して働きかける画面がどの投稿も名指さないためで
// あり、その「投稿は無い」をレス番号0で表さないためです。スレッドの番号は1から始まるため、
// 壊れたアドレスから届いた0は、そうしなければスレッドが持たない投稿としてではなく「投稿は
// 無い」として読まれてしまいます。
type GetThreadModerationInput struct {
	Actor    Actor
	ThreadID model.ThreadID
	Number   *int
}

// GetThreadModerationOutputは、確認ページが自身の対象とする操作の相手として名指す
// ものです。スレッドと、画面が投稿1件を対象とするときはその投稿です。スレッド自身に対して
// 働きかける画面ではPostがnilになります。
//
// PostAuthorはその投稿を書いたアカウントで、スレッドに対して働きかける画面でも、作者を
// 解決できない投稿 (アカウントが退会したか、その行が既にパージされたか) でもnilです。
// どちらの場合も画面は作者を名指します。管理者の前に置かれるのが、コミュニティの読むとおりの
// 投稿であるためです。
type GetThreadModerationOutput struct {
	Thread     *model.Thread
	Post       *model.Post
	PostAuthor *model.User
}

// GetThreadModerationUsecaseは、スレッドの下の確認ページが描かれる元を読みます。
// 読み取りUseCaseであり、リポジトリの取得系メソッドしか呼ばないため、validatorも
// トランザクションも必要としません。
//
// ここが解決するのは、何かが行われる前に画面が示す対象です。その操作が実際に行われるか
// どうかを答えるのは書き込みUseCaseであり、それは自身のトランザクションの中で読み直した
// 状態に対して答えます。したがって、このUseCaseが報告したことが送信の届く時点でも成立して
// いることは当てにしません。
type GetThreadModerationUsecase struct {
	roleRepo   *repository.RoleRepository
	threadRepo *repository.ThreadRepository
	postRepo   *repository.PostRepository
	userRepo   *repository.UserRepository
}

// NewGetThreadModerationUsecaseは、確認ページが読み取る各リポジトリから
// GetThreadModerationUsecaseを構築します。
func NewGetThreadModerationUsecase(
	roleRepo *repository.RoleRepository,
	threadRepo *repository.ThreadRepository,
	postRepo *repository.PostRepository,
	userRepo *repository.UserRepository,
) *GetThreadModerationUsecase {
	return &GetThreadModerationUsecase{
		roleRepo:   roleRepo,
		threadRepo: threadRepo,
		postRepo:   postRepo,
		userRepo:   userRepo,
	}
}

// Executeは確認ページが名指す対象を解決します。
//
// 権限を最初に答えるのは、アドレスがどのスレッドも名指していないことも、投稿がもう示されて
// いないことも、これらの画面を許された人だけが知ることであるようにするためです。尋ねるのは、
// 操作者がそもそもスレッドに対して働きかけてよいかどうかであって、この特定の操作を行って
// よいかどうかではありません。画面は操作の1つ手前にあり、その操作の判定は操作自身の手前に
// 立っています。
//
// 非公開のスレッドは、各操作に対してそうであるのと同じくAppErrCodeResourceUnpublishedです。
// 確認ページは、コミュニティがもう示していないものを対象として名指すことになるためです。
//
// 非公開の投稿を、そのスレッドが受け取る答えではなくAppErrCodeResourceNotFoundとするのは、
// それを名指すページに示すものが無いためです。そうしたページが管理者の前に置いて判断の材料と
// するのは投稿の本文であり、そこに付いた印はその本文を視界の外へ移しています。
func (uc *GetThreadModerationUsecase) Execute(ctx context.Context, input GetThreadModerationInput) (*GetThreadModerationOutput, error) {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return nil, err
	}
	if !communityPolicy.CanModerateThread() {
		return nil, forbiddenThreadModeration(ctx, "スレッドのモデレーションの画面を開く権限がない", input.ThreadID)
	}

	thread, err := findModeratedThread(ctx, uc.threadRepo, input.ThreadID)
	if err != nil {
		return nil, err
	}
	if input.Number == nil {
		return &GetThreadModerationOutput{Thread: thread}, nil
	}

	post, err := findExistingPost(ctx, uc.postRepo, thread.ID, *input.Number)
	if err != nil {
		return nil, err
	}
	if post.UnpublishedAt != nil {
		return nil, missingPost(ctx, thread.ID, *input.Number)
	}

	author, err := uc.findPostAuthor(ctx, post)
	if err != nil {
		return nil, err
	}

	return &GetThreadModerationOutput{Thread: thread, Post: post, PostAuthor: author}, nil
}

// findPostAuthorは投稿を書いたアカウントを読み、解決できるものが無いときはnilを
// 返します。作者が退会した投稿も投稿であるため、画面は、他の点では示せる対象を名指すことを
// 拒むのではなく、作者が居なくなったことを述べます。
func (uc *GetThreadModerationUsecase) findPostAuthor(ctx context.Context, post *model.Post) (*model.User, error) {
	if post.UserID == nil {
		return nil, nil
	}

	author, err := uc.userRepo.FindByID(ctx, *post.UserID)
	if err != nil {
		return nil, fmt.Errorf("投稿の作者の取得に失敗: %w", err)
	}
	return author, nil
}
