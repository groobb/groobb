package usecase

import (
	"context"
	"errors"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetAdminHomeUsecaseは、操作者が管理ハブ、すなわちコミュニティの管理画面を
// 並べるページを開いてよいかどうかを答えます。
//
// ハブはコミュニティについて何も読みません。ハブが持つのはリンクであり、その先の各画面が
// そこで操作者に何を見せるかを自分で決めます。そのため本UseCaseは出力を持たず、
// そもそもページを描画するかどうかだけを答えます。ハンドラーがポリシーを直接尋ねるのでは
// なくこれを置くのは、認可がUseCaseの受け持ちであるためです。ハンドラーには、どのページに
// ついてのものかを述べる仕事だけが残ります (アーキテクチャガイド)。
type GetAdminHomeUsecase struct {
	roleRepo *repository.RoleRepository
}

// NewGetAdminHomeUsecaseは、操作者の権限を解決するために使うロールのリポジトリから
// GetAdminHomeUsecaseを構築します。
func NewGetAdminHomeUsecase(roleRepo *repository.RoleRepository) *GetAdminHomeUsecase {
	return &GetAdminHomeUsecase{roleRepo: roleRepo}
}

// GetAdminHomeInputはExecuteの入力で、誰がハブを開こうとしているかを表します。
type GetAdminHomeInput struct {
	Actor Actor
}

// Executeは、操作者が管理ハブを開いてよいかどうかを答え、開いてよくないときは
// AppErrCodeForbiddenのAppErrorを返します。
//
// ハブはどれか1つの管理画面を許された人に許されます。コミュニティの管理者より狭いロールを
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
