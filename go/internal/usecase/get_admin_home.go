package usecase

import (
	"context"
	"errors"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetAdminHomeUsecase answers whether the actor may open the admin hub, the page
// the community's administration screens are listed on.
//
// The hub reads nothing of the community: what it holds is links, and each screen
// behind them decides for itself what the actor may see there. So this UseCase
// carries no output and reports only whether the page is to be rendered at all.
// It exists rather than the Handler asking the policy directly because authorizing
// is the UseCase's part of the work: the Handler is left saying which page it is
// about (see the architecture guide).
//
// [Ja] GetAdminHomeUsecase は、操作者が管理ハブ、すなわちコミュニティの管理画面を
// 並べるページを開いてよいかどうかを答えます。
//
// ハブはコミュニティについて何も読みません。ハブが持つのはリンクであり、その先の各画面が
// そこで操作者に何を見せるかを自分で決めます。そのため本 UseCase は出力を持たず、
// そもそもページを描画するかどうかだけを答えます。ハンドラーがポリシーを直接尋ねるのでは
// なくこれを置くのは、認可が UseCase の受け持ちであるためです。ハンドラーには、どのページに
// ついてのものかを述べる仕事だけが残ります (アーキテクチャガイド)。
type GetAdminHomeUsecase struct {
	roleRepo *repository.RoleRepository
}

// NewGetAdminHomeUsecase builds a GetAdminHomeUsecase over the role repository it
// resolves the actor's permission through.
//
// [Ja] NewGetAdminHomeUsecase は、操作者の権限を解決するために使うロールのリポジトリから
// GetAdminHomeUsecase を構築します。
func NewGetAdminHomeUsecase(roleRepo *repository.RoleRepository) *GetAdminHomeUsecase {
	return &GetAdminHomeUsecase{roleRepo: roleRepo}
}

// GetAdminHomeInput is the input to Execute: who is opening the hub.
//
// [Ja] GetAdminHomeInput は Execute の入力で、誰がハブを開こうとしているかを表します。
type GetAdminHomeInput struct {
	Actor Actor
}

// Execute reports whether the actor may open the admin hub, returning an
// AppErrCodeForbidden AppError when they may not.
//
// The hub is admitted to anyone admitted to any one of the administration
// screens, so that an actor who holds a narrower role than the community's
// administrator still reaches the screens they do hold.
//
// [Ja] Execute は、操作者が管理ハブを開いてよいかどうかを答え、開いてよくないときは
// AppErrCodeForbidden の AppError を返します。
//
// ハブはどれか 1 つの管理画面を許された人に許されます。コミュニティの管理者より狭いロールを
// 持つ操作者も、自分が持つ画面には辿り着けるようにするためです。
func (uc *GetAdminHomeUsecase) Execute(ctx context.Context, input GetAdminHomeInput) error {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return err
	}
	if !communityPolicy.CanAccessAdmin() {
		return &model.AppError{
			Code:     model.AppErrCodeForbidden,
			UserMsg:  i18n.T(ctx, "error_forbidden_message"),
			Internal: errors.New("管理画面を開く権限がない"),
		}
	}

	return nil
}
