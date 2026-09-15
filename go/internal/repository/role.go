package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// RoleRepositoryはsqlc生成のクエリ経由でrolesを読みます。
//
// 書き込みのメソッドを持ちません。インスタンスが持つロールはマイグレーションが作ります。
// 何も実行されていないインスタンスにも管理者を立てられるようにするためです。実行時に
// 変わるのは誰がロールを持つかであり、それはUserRoleRepositoryが担います。
type RoleRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewRoleRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで
// 書くRoleRepositoryを生成します。
func NewRoleRepository(db *database.DB) *RoleRepository {
	return &RoleRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいRoleRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *RoleRepository) WithTx(tx *sql.Tx) *RoleRepository {
	q := r.writer.WithTx(tx)
	return &RoleRepository{reader: q, writer: q}
}

// FindByNameは指定した名前のロールを返し、存在しない場合は (nil, nil) を返します。
// ロールへidではなく名前で辿り着くのは、idがマイグレーションを適用したデータベースごとの
// ものである一方、名前はどのインスタンスでも同じであるためです。
//
// どのロールも持たない名前は正常なルックアップ結果でありエラーではありません。運用者が
// 存在しないロールを打ち込んだときにコマンドラインが渡してくるのがそれです。
func (r *RoleRepository) FindByName(ctx context.Context, name model.RoleName) (*model.Role, error) {
	row, err := r.reader.GetRoleByName(ctx, string(name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row)
}

// ListByUserIDは指定したユーザーが持つロールをid順で返します。そのユーザーが
// 何を許されるかは、それらすべてのスコープを合わせたものであるため、ポリシーを組み立てる
// 呼び出し側は1つのロールではなくスライス全体を受け取ります。
//
// ロールを1つも持たないユーザーには空のスライスを返します。何も持たないことは大半の人が
// 置かれている状態であり、報告すべき欠落ではありません。
func (r *RoleRepository) ListByUserID(ctx context.Context, userID model.UserID) ([]*model.Role, error) {
	rows, err := r.reader.ListRolesByUserID(ctx, int64(userID))
	if err != nil {
		return nil, err
	}

	roles := make([]*model.Role, len(rows))
	for i, row := range rows {
		role, err := r.toModel(row)
		if err != nil {
			return nil, err
		}
		roles[i] = role
	}
	return roles, nil
}

// ListByUserIDsは、指定したいずれかのユーザーが持つロールを、それを持つユーザーを
// キーとし、ユーザーごとにはロールのid順で返します。idを1つずつではなくまとめて取る
// のは、そうしなければ1ページ分の利用者を並べるページで1行につき1クエリになるためです。
//
// ロールを1つも持たないユーザーは、空のスライスを伴って現れるのではなくマップから欠けます。
// これはそのユーザー1人を引いたときと同じ答えです。欠けたキーのゼロ値が、呼び出し側が
// ループする空のスライスそのものであるためです。
//
// 空のidスライスに対してはクエリを発行せず空のマップを返します。ロールを引く手がかりが
// 無いためです。
func (r *RoleRepository) ListByUserIDs(ctx context.Context, userIDs []model.UserID) (map[model.UserID][]*model.Role, error) {
	if len(userIDs) == 0 {
		return map[model.UserID][]*model.Role{}, nil
	}

	rawIDs := make([]int64, len(userIDs))
	for i, id := range userIDs {
		rawIDs[i] = int64(id)
	}

	rows, err := r.reader.ListRolesByUserIDs(ctx, rawIDs)
	if err != nil {
		return nil, err
	}

	rolesByUserID := make(map[model.UserID][]*model.Role, len(userIDs))
	for _, row := range rows {
		role, err := r.toModel(row.Role)
		if err != nil {
			return nil, err
		}
		userID := model.UserID(row.UserID)
		rolesByUserID[userID] = append(rolesByUserID[userID], role)
	}
	return rolesByUserID, nil
}

// toModelはquery.Roleをmodel.Roleに変換し、リポジトリの境界で生のidを型付きの
// 形に、保存書式の時刻をtime.Timeにキャストします。scopes列が文字列のJSON配列を
// 保持していない場合はエラーを返します。それは列自身のチェック制約が許してしまう状態です。
func (r *RoleRepository) toModel(row query.Role) (*model.Role, error) {
	scopes, err := decodeScopes(row.Scopes)
	if err != nil {
		return nil, err
	}

	return &model.Role{
		ID:        model.RoleID(row.ID),
		Name:      model.RoleName(row.Name),
		Scopes:    scopes,
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}, nil
}

// decodeScopesはscopes列が保存するJSON配列をスコープに解釈し直します。
//
// 配列の中の名前は、語彙が定義していないものも含めてすべて保ちます。ここで落とすと、
// バイナリより先にマイグレートされたデータベースは、古いビルドがロールを読んで何かが
// 書き戻した時点で、持っていたものを失います。知らない名前が何であるかはスコープを判定する
// 場所が決めることであり、そこでは何も与えません。
func decodeScopes(encoded string) ([]model.Scope, error) {
	var names []string
	if err := json.Unmarshal([]byte(encoded), &names); err != nil {
		return nil, fmt.Errorf("failed to decode the role scopes: %w", err)
	}

	scopes := make([]model.Scope, len(names))
	for i, name := range names {
		scopes[i] = model.Scope(name)
	}
	return scopes, nil
}
