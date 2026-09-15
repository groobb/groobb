package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// GetAdminUsersInputはExecuteの入力です。誰が一覧を読むか、それを何で絞り込んで
// いるか、そのどのページを見ているかを表します。
//
// AtnamePrefixは何も検索していないとき空になり、Pageは1から数えます。1はアドレスが
// 最初のページに与える番号です。
type GetAdminUsersInput struct {
	Actor        Actor
	AtnamePrefix string
	Page         int
}

// AdminUserは管理画面の利用者一覧の1行、すなわち1つのアカウントと、その
// アカウントが持つロールです。ロールが伴うのは、一覧が開かれる目的がそれを見て変えること
// であり、行ごとに読めば表示する人数だけクエリを払うことになるためです。
type AdminUser struct {
	User  *model.User
	Roles []*model.Role
}

// GetAdminUsersOutputは一覧の1ページと、その一覧全体が何件を対象とするかです。
// 後者が、下に並ぶページの番号の元になります。
type GetAdminUsersOutput struct {
	Users      []AdminUser
	TotalCount int
}

// GetAdminUsersUsecaseは、管理画面の利用者一覧が描かれる元となる、コミュニティの
// アカウントの1ページを読みます。読み取りUseCaseであり、リポジトリの取得系メソッド
// しか呼ばないため、validatorもトランザクションも必要としません。
type GetAdminUsersUsecase struct {
	roleRepo *repository.RoleRepository
	userRepo *repository.UserRepository
}

// NewGetAdminUsersUsecaseは、一覧が読み取る各リポジトリから
// GetAdminUsersUsecaseを構築します。
func NewGetAdminUsersUsecase(
	roleRepo *repository.RoleRepository,
	userRepo *repository.UserRepository,
) *GetAdminUsersUsecase {
	return &GetAdminUsersUsecase{roleRepo: roleRepo, userRepo: userRepo}
}

// Executeは、求められた一覧のページを読みます。
//
// 権限を最初に答えるのは、どのページにも振られていない番号も、何にも一致しない検索も、
// 一覧を許された人だけが知ることであるようにするためです。
//
// 最初のページより前のページはAppErrCodeResourceNotFoundです。ページ番号はアドレスの
// 一部であり、一覧が決して振らない番号はどのページも名指していません。最後のページより
// 後ろの番号はこれとは別のもので、番号がどこまで届くかはその時点のアカウントの数で決まる
// ためです。それは、アカウントを持つ一覧の空のページであり、最後を通り過ぎてページを送った
// 一覧の姿そのものです。
func (uc *GetAdminUsersUsecase) Execute(ctx context.Context, input GetAdminUsersInput) (*GetAdminUsersOutput, error) {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return nil, err
	}
	if !communityPolicy.CanListUsers() {
		return nil, &model.AppError{
			Code:     model.AppErrCodeForbidden,
			UserMsg:  i18n.T(ctx, "error_forbidden_message"),
			Internal: errors.New("利用者の一覧を読む権限がない"),
		}
	}

	if input.Page < 1 {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("最初のページより前のページ番号: page=%d", input.Page),
			Metadata: map[string]string{"page": strconv.Itoa(input.Page)},
		}
	}

	// atnameの文字集合の外にある検索は何にも一致しないため、一覧はデータベースに
	// 尋ねずにそう答えます。その値を持てるアカウントは無く、クエリは空で返るためだけに
	// 発行されることになります。
	if input.AtnamePrefix != "" && !validator.IsValidAtname(input.AtnamePrefix) {
		return &GetAdminUsersOutput{Users: []AdminUser{}}, nil
	}

	return uc.listPage(ctx, input)
}

// listPageはページと、そこに載る人々のロールを読みます。
//
// 件数を先に読むのは、それがページに何かが載りうるかどうかを述べるためです。最後のページ
// より後ろのページは、空で返る読み取りを行わずに答えます。ページ番号は掛け合わせるのでは
// なく件数と比べます。どの一覧よりもはるかに大きな番号が、桁あふれによって一覧の内側の
// オフセットに化けることがないようにするためです。
func (uc *GetAdminUsersUsecase) listPage(ctx context.Context, input GetAdminUsersInput) (*GetAdminUsersOutput, error) {
	totalCount, err := uc.userRepo.CountByAtnamePrefix(ctx, input.AtnamePrefix)
	if err != nil {
		return nil, fmt.Errorf("利用者の件数の取得に失敗: %w", err)
	}

	pagesBefore := input.Page - 1
	if totalCount == 0 || pagesBefore > (totalCount-1)/model.AdminUsersPerPage {
		return &GetAdminUsersOutput{Users: []AdminUser{}, TotalCount: totalCount}, nil
	}

	users, err := uc.userRepo.ListPageByAtnamePrefix(
		ctx,
		input.AtnamePrefix,
		model.AdminUsersPerPage,
		pagesBefore*model.AdminUsersPerPage,
	)
	if err != nil {
		return nil, fmt.Errorf("利用者一覧の取得に失敗: %w", err)
	}

	userIDs := make([]model.UserID, len(users))
	for i, user := range users {
		userIDs[i] = user.ID
	}
	rolesByUserID, err := uc.roleRepo.ListByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("利用者のロールの取得に失敗: %w", err)
	}

	rows := make([]AdminUser, len(users))
	for i, user := range users {
		rows[i] = AdminUser{User: user, Roles: rolesByUserID[user.ID]}
	}

	return &GetAdminUsersOutput{Users: rows, TotalCount: totalCount}, nil
}
