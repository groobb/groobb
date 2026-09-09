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

// GetAdminUsersInput is the input to Execute: who is reading the listing, what
// they are narrowing it to, and which page of it they are on.
//
// AtnamePrefix is empty when nothing is being searched for, and Page counts from
// 1, which is the number the first page carries in the address.
//
// [Ja] GetAdminUsersInput は Execute の入力です。誰が一覧を読むか、それを何で絞り込んで
// いるか、そのどのページを見ているかを表します。
//
// AtnamePrefix は何も検索していないとき空になり、Page は 1 から数えます。1 はアドレスが
// 最初のページに与える番号です。
type GetAdminUsersInput struct {
	Actor        Actor
	AtnamePrefix string
	Page         int
}

// AdminUser is one row of the admin user listing: an account, and the roles that
// account holds. The roles come along because what the listing is opened for is
// to see and change them, and reading them per row would cost a query per
// person shown.
//
// [Ja] AdminUser は管理画面の利用者一覧の 1 行、すなわち 1 つのアカウントと、その
// アカウントが持つロールです。ロールが伴うのは、一覧が開かれる目的がそれを見て変えること
// であり、行ごとに読めば表示する人数だけクエリを払うことになるためです。
type AdminUser struct {
	User  *model.User
	Roles []*model.Role
}

// GetAdminUsersOutput is one page of the listing together with how many accounts
// the whole listing covers, which is what the pages under it are numbered from.
//
// [Ja] GetAdminUsersOutput は一覧の 1 ページと、その一覧全体が何件を対象とするかです。
// 後者が、下に並ぶページの番号の元になります。
type GetAdminUsersOutput struct {
	Users      []AdminUser
	TotalCount int
}

// GetAdminUsersUsecase reads the page of the community's accounts the admin
// user listing is drawn from. It is a read UseCase: it only calls the lookup
// methods of its repositories, so it needs neither a validator nor a
// transaction.
//
// [Ja] GetAdminUsersUsecase は、管理画面の利用者一覧が描かれる元となる、コミュニティの
// アカウントの 1 ページを読みます。読み取り UseCase であり、リポジトリの取得系メソッド
// しか呼ばないため、validator もトランザクションも必要としません。
type GetAdminUsersUsecase struct {
	roleRepo *repository.RoleRepository
	userRepo *repository.UserRepository
}

// NewGetAdminUsersUsecase builds a GetAdminUsersUsecase over the repositories
// the listing is read from.
//
// [Ja] NewGetAdminUsersUsecase は、一覧が読み取る各リポジトリから
// GetAdminUsersUsecase を構築します。
func NewGetAdminUsersUsecase(
	roleRepo *repository.RoleRepository,
	userRepo *repository.UserRepository,
) *GetAdminUsersUsecase {
	return &GetAdminUsersUsecase{roleRepo: roleRepo, userRepo: userRepo}
}

// Execute reads the requested page of the listing.
//
// Permission is answered first, so that a page number nothing is numbered by and
// a search nothing can match are both things only someone admitted to the
// listing learns about.
//
// A page below the first one is AppErrCodeResourceNotFound: a page number is
// part of the address, and one the listing is never numbered by names no page.
// A number past the last page is not the same thing, because how far the
// numbering reaches depends on how many accounts there are at the moment: it is
// an empty page of a listing that has them, which is what a listing someone has
// paged past the end of looks like.
//
// [Ja] Execute は、求められた一覧のページを読みます。
//
// 権限を最初に答えるのは、どのページにも振られていない番号も、何にも一致しない検索も、
// 一覧を許された人だけが知ることであるようにするためです。
//
// 最初のページより前のページは AppErrCodeResourceNotFound です。ページ番号はアドレスの
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

	// A search outside the atname character set matches nothing, and the
	// listing says so without asking the database: no account can hold such a
	// value, so the query would be issued only to come back empty.
	//
	// [Ja] atname の文字集合の外にある検索は何にも一致しないため、一覧はデータベースに
	// 尋ねずにそう答えます。その値を持てるアカウントは無く、クエリは空で返るためだけに
	// 発行されることになります。
	if input.AtnamePrefix != "" && !validator.IsValidAtname(input.AtnamePrefix) {
		return &GetAdminUsersOutput{Users: []AdminUser{}}, nil
	}

	return uc.listPage(ctx, input)
}

// listPage reads the page and the roles of the people on it.
//
// The count comes first because it is what says whether the page can hold
// anything: a page past the last one is answered without the read that would
// come back empty, and the page number is compared against the count rather than
// multiplied out, so a number far larger than any listing cannot overflow into
// an offset that lands back inside it.
//
// [Ja] listPage はページと、そこに載る人々のロールを読みます。
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
