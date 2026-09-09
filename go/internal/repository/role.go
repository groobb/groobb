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

// RoleRepository reads roles through sqlc-generated queries.
//
// It has no write methods: the roles an instance has are created by migrations,
// so that an instance can be given an administrator before anything has run
// against it. What changes at run time is who holds a role, which is
// UserRoleRepository's subject.
//
// [Ja] RoleRepository は sqlc 生成のクエリ経由で roles を読みます。
//
// 書き込みのメソッドを持ちません。インスタンスが持つロールはマイグレーションが作ります。
// 何も実行されていないインスタンスにも管理者を立てられるようにするためです。実行時に
// 変わるのは誰がロールを持つかであり、それは UserRoleRepository が担います。
type RoleRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewRoleRepository creates a RoleRepository that reads through the database's
// read pool and writes through its write pool.
//
// [Ja] NewRoleRepository は、データベースの読み取り用プールで読み、書き込み用プールで
// 書く RoleRepository を生成します。
func NewRoleRepository(db *database.DB) *RoleRepository {
	return &RoleRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new RoleRepository whose queries run inside tx, so a UseCase
// can enlist this repository in its transaction. The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい RoleRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *RoleRepository) WithTx(tx *sql.Tx) *RoleRepository {
	q := r.writer.WithTx(tx)
	return &RoleRepository{reader: q, writer: q}
}

// FindByName returns the role with the given name, or (nil, nil) when none
// exists. A role is reached by name rather than by id because the id belongs to
// whichever database the migration ran against, while the name is the same
// across every instance.
//
// A name nothing carries is a normal lookup outcome, not an error: it is what a
// command line hands over when the operator typed a role that does not exist.
//
// [Ja] FindByName は指定した名前のロールを返し、存在しない場合は (nil, nil) を返します。
// ロールへ id ではなく名前で辿り着くのは、id がマイグレーションを適用したデータベースごとの
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

// ListByUserID returns the roles the given user holds, in id order. What the
// user is admitted to is the scopes of all of them taken together, so the caller
// building a policy takes the whole slice rather than one role.
//
// A user holding no role gets an empty slice: holding nothing is the state most
// people are in, not an absence to report.
//
// [Ja] ListByUserID は指定したユーザーが持つロールを id 順で返します。そのユーザーが
// 何を許されるかは、それらすべてのスコープを合わせたものであるため、ポリシーを組み立てる
// 呼び出し側は 1 つのロールではなくスライス全体を受け取ります。
//
// ロールを 1 つも持たないユーザーには空のスライスを返します。何も持たないことは大半の人が
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

// ListByUserIDs returns the roles held by any of the given users, keyed by the
// user holding them and in role id order within each user. It takes the ids
// together rather than one at a time because the alternative is one query per
// row on a page listing a whole page of people.
//
// A user holding no role is absent from the map rather than present with an
// empty slice, which is the same answer a lookup of that user alone gives: the
// zero value of the missing key is the empty slice a caller ranges over.
//
// An empty slice of ids returns an empty map without querying: there is nothing
// to look roles up by.
//
// [Ja] ListByUserIDs は、指定したいずれかのユーザーが持つロールを、それを持つユーザーを
// キーとし、ユーザーごとにはロールの id 順で返します。id を 1 つずつではなくまとめて取る
// のは、そうしなければ 1 ページ分の利用者を並べるページで 1 行につき 1 クエリになるためです。
//
// ロールを 1 つも持たないユーザーは、空のスライスを伴って現れるのではなくマップから欠けます。
// これはそのユーザー 1 人を引いたときと同じ答えです。欠けたキーのゼロ値が、呼び出し側が
// ループする空のスライスそのものであるためです。
//
// 空の id スライスに対してはクエリを発行せず空のマップを返します。ロールを引く手がかりが
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

// toModel converts a query.Role row into a model.Role, casting the raw id into
// its typed form and the stored timestamps back into time.Time at the repository
// boundary. It returns an error when the scopes column does not hold a JSON
// array of strings, which the column's own check constraint leaves open.
//
// [Ja] toModel は query.Role を model.Role に変換し、リポジトリの境界で生の id を型付きの
// 形に、保存書式の時刻を time.Time にキャストします。scopes 列が文字列の JSON 配列を
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

// decodeScopes parses the JSON array the scopes column stores back into scopes.
//
// Every name in the array is kept, including one the vocabulary does not define.
// Dropping such a name here would make a database migrated ahead of the binary
// lose what it holds as soon as an older build reads the role and something
// writes it back; what an unknown name amounts to is decided where scopes are
// checked, and there it grants nothing.
//
// [Ja] decodeScopes は scopes 列が保存する JSON 配列をスコープに解釈し直します。
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
