package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// UserRoleRepositoryはsqlc生成のクエリ経由でuser_rolesを読み書きします。
type UserRoleRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserRoleRepositoryは、データベースの読み取り用プールで読み、書き込み用
// プールで書くUserRoleRepositoryを生成します。
func NewUserRoleRepository(db *database.DB) *UserRoleRepository {
	return &UserRoleRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいUserRoleRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *UserRoleRepository) WithTx(tx *sql.Tx) *UserRoleRepository {
	q := r.writer.WithTx(tx)
	return &UserRoleRepository{reader: q, writer: q}
}

// CreateUserRoleInputはロールの割当に必要な属性を保持します。idとタイムスタンプは
// DB側で採番されます。
type CreateUserRoleInput struct {
	UserID model.UserID
	RoleID model.RoleID
}

// Createはロールをユーザーへ割り当て、DBが採番したidとタイムスタンプを設定した
// 状態でその割当を返します。ユーザーが既に持つロールの割当はUNIQUE (user_id, role_id) が
// 拒否します。これが、同時に何件のリクエストが届いても1人が1つのロールを二重に持つことを
// 防ぎます。その競合に負けることが通常の結果である呼び出し側は、IsUniqueViolationで
// それを見分けます。
func (r *UserRoleRepository) Create(ctx context.Context, input CreateUserRoleInput) (*model.UserRole, error) {
	row, err := r.writer.CreateUserRole(ctx, query.CreateUserRoleParams{
		UserID: int64(input.UserID),
		RoleID: int64(input.RoleID),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// ListByUserIDsは指定したいずれかのユーザーが持つ割当を、ユーザーごとにまとまった
// 形で、その中ではロールid順に返します。idを1つずつではなくまとめて取るのは、
// そうしなければ1ページ分の利用者を並べるページで1行につき1クエリになるためです。
//
// 順序をエンジンに委ねず明示するのは、エンジンがたまたま採った計画の順で行を返してよい
// ためです。
//
// 空のidスライスに対してはクエリを発行せず空のスライスを返します。割当を引く手がかりが
// 無いためです。
func (r *UserRoleRepository) ListByUserIDs(ctx context.Context, userIDs []model.UserID) ([]*model.UserRole, error) {
	if len(userIDs) == 0 {
		return []*model.UserRole{}, nil
	}

	rawIDs := make([]int64, len(userIDs))
	for i, id := range userIDs {
		rawIDs[i] = int64(id)
	}

	rows, err := r.reader.ListUserRolesByUserIDs(ctx, rawIDs)
	if err != nil {
		return nil, err
	}

	userRoles := make([]*model.UserRole, len(rows))
	for i, row := range rows {
		userRoles[i] = r.toModel(row)
	}
	return userRoles, nil
}

// CountHoldersByRoleIDは、そのロールを何人が持っているかを、まだそのロールで行動
// できる人だけを数えて返します。退会したユーザーはアカウントが完全に削除されるまで割当を
// 保ち、停止されたユーザーも停止が続く間は割当を保ちます。どちらを数えても、誰もサイン
// インできない管理者だけがコミュニティに残りえます。
//
// あるロールを1人以上が持つ状態を保たなければならない呼び出し側は、この件数を書き込む
// トランザクションの中で読み、判断の対象にした数がその足元で変わらないようにします。
func (r *UserRoleRepository) CountHoldersByRoleID(ctx context.Context, roleID model.RoleID) (int, error) {
	count, err := r.reader.CountUserRoleHoldersByRoleID(ctx, int64(roleID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// DeleteByUserIDAndRoleIDは、指定したロールに対するユーザーの割当を削除します。
// ユーザーが持たないロールの削除はエラーにならないため、2度剥奪しても1度剥奪したのと
// 同じ状態になります。
func (r *UserRoleRepository) DeleteByUserIDAndRoleID(ctx context.Context, userID model.UserID, roleID model.RoleID) error {
	return r.writer.DeleteUserRoleByUserIDAndRoleID(ctx, query.DeleteUserRoleByUserIDAndRoleIDParams{
		UserID: int64(userID),
		RoleID: int64(roleID),
	})
}

// DeleteByUserIDは指定したユーザーが持つロールをすべて削除します。退会がこれを
// 使い、コミュニティを去るアカウントが、持っていたものの保持者として数えられ続けない
// ようにします。ユーザーが何も持たないときに削除してもエラーにならないため、呼び出し側で
// 事前確認は不要です。
func (r *UserRoleRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUserRolesByUserID(ctx, int64(userID))
}

// toModelはquery.UserRoleをmodel.UserRoleに変換し、リポジトリの境界で生のidを
// 型付きの形に、保存書式の時刻をtime.Timeにキャストします。
func (r *UserRoleRepository) toModel(row query.UserRole) *model.UserRole {
	return &model.UserRole{
		ID:        model.UserRoleID(row.ID),
		UserID:    model.UserID(row.UserID),
		RoleID:    model.RoleID(row.RoleID),
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
