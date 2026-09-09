package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// UserRoleRepository reads and writes role assignments through sqlc-generated
// queries.
//
// [Ja] UserRoleRepository は sqlc 生成のクエリ経由で user_roles を読み書きします。
type UserRoleRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserRoleRepository creates a UserRoleRepository that reads through the
// database's read pool and writes through its write pool.
//
// [Ja] NewUserRoleRepository は、データベースの読み取り用プールで読み、書き込み用
// プールで書く UserRoleRepository を生成します。
func NewUserRoleRepository(db *database.DB) *UserRoleRepository {
	return &UserRoleRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new UserRoleRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserRoleRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *UserRoleRepository) WithTx(tx *sql.Tx) *UserRoleRepository {
	q := r.writer.WithTx(tx)
	return &UserRoleRepository{reader: q, writer: q}
}

// CreateUserRoleInput holds the attributes needed to assign a role. id and the
// timestamps are assigned by the database.
//
// [Ja] CreateUserRoleInput はロールの割当に必要な属性を保持します。id とタイムスタンプは
// DB 側で採番されます。
type CreateUserRoleInput struct {
	UserID model.UserID
	RoleID model.RoleID
}

// Create assigns the role to the user and returns the assignment with the
// database-assigned id and timestamps populated. Assigning a role the user
// already holds is rejected by UNIQUE (user_id, role_id), which is what keeps
// one person from holding one role twice however many requests arrive at once;
// a caller for whom losing that race is a normal outcome recognizes it with
// IsUniqueViolation.
//
// [Ja] Create はロールをユーザーへ割り当て、DB が採番した id とタイムスタンプを設定した
// 状態でその割当を返します。ユーザーが既に持つロールの割当は UNIQUE (user_id, role_id) が
// 拒否します。これが、同時に何件のリクエストが届いても 1 人が 1 つのロールを二重に持つことを
// 防ぎます。その競合に負けることが通常の結果である呼び出し側は、IsUniqueViolation で
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

// ListByUserIDs returns the assignments held by any of the given users, grouped
// by user and, within a group, in role id order. It takes the ids together
// rather than one at a time because the alternative is one query per row on a
// page listing a whole page of people.
//
// Ordering is stated rather than left to the engine, which is free to return the
// rows in whichever order the plan happens to produce.
//
// An empty slice of ids returns an empty slice without querying: there is
// nothing to look assignments up by.
//
// [Ja] ListByUserIDs は指定したいずれかのユーザーが持つ割当を、ユーザーごとにまとまった
// 形で、その中ではロール id 順に返します。id を 1 つずつではなくまとめて取るのは、
// そうしなければ 1 ページ分の利用者を並べるページで 1 行につき 1 クエリになるためです。
//
// 順序をエンジンに委ねず明示するのは、エンジンがたまたま採った計画の順で行を返してよい
// ためです。
//
// 空の id スライスに対してはクエリを発行せず空のスライスを返します。割当を引く手がかりが
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

// CountHoldersByRoleID returns how many people hold the role, counting only
// those who have not withdrawn. A withdrawn user keeps their assignments until
// the account is purged, and counting them would let the community be left with
// an administrator nobody can sign in as.
//
// A caller that has to keep a role held by at least one person reads this count
// inside the transaction that writes, so that the number it acts on cannot
// change under it.
//
// [Ja] CountHoldersByRoleID は、そのロールを何人が持っているかを、退会していない人だけを
// 数えて返します。退会したユーザーはアカウントが完全に削除されるまで割当を保つため、
// その人を数えると、誰もサインインできない管理者だけがコミュニティに残りえます。
//
// あるロールを 1 人以上が持つ状態を保たなければならない呼び出し側は、この件数を書き込む
// トランザクションの中で読み、判断の対象にした数がその足元で変わらないようにします。
func (r *UserRoleRepository) CountHoldersByRoleID(ctx context.Context, roleID model.RoleID) (int, error) {
	count, err := r.reader.CountUserRoleHoldersByRoleID(ctx, int64(roleID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// DeleteByUserIDAndRoleID removes the user's assignment of the given role.
// Removing a role the user does not hold is not an error, so revoking twice
// leaves the same state as revoking once.
//
// [Ja] DeleteByUserIDAndRoleID は、指定したロールに対するユーザーの割当を削除します。
// ユーザーが持たないロールの削除はエラーにならないため、2 度剥奪しても 1 度剥奪したのと
// 同じ状態になります。
func (r *UserRoleRepository) DeleteByUserIDAndRoleID(ctx context.Context, userID model.UserID, roleID model.RoleID) error {
	return r.writer.DeleteUserRoleByUserIDAndRoleID(ctx, query.DeleteUserRoleByUserIDAndRoleIDParams{
		UserID: int64(userID),
		RoleID: int64(roleID),
	})
}

// DeleteByUserID removes every role the given user holds. Withdrawal uses it so
// that an account leaving the community stops being counted among the holders of
// what it held. Deleting when the user holds nothing is not an error, so callers
// need not check first.
//
// [Ja] DeleteByUserID は指定したユーザーが持つロールをすべて削除します。退会がこれを
// 使い、コミュニティを去るアカウントが、持っていたものの保持者として数えられ続けない
// ようにします。ユーザーが何も持たないときに削除してもエラーにならないため、呼び出し側で
// 事前確認は不要です。
func (r *UserRoleRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUserRolesByUserID(ctx, int64(userID))
}

// toModel converts a query.UserRole row into a model.UserRole, casting the raw
// ids into their typed forms and the stored timestamps back into time.Time at
// the repository boundary.
//
// [Ja] toModel は query.UserRole を model.UserRole に変換し、リポジトリの境界で生の id を
// 型付きの形に、保存書式の時刻を time.Time にキャストします。
func (r *UserRoleRepository) toModel(row query.UserRole) *model.UserRole {
	return &model.UserRole{
		ID:        model.UserRoleID(row.ID),
		UserID:    model.UserID(row.UserID),
		RoleID:    model.RoleID(row.RoleID),
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
