package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// CommunityRepository reads and writes the community through sqlc-generated
// queries.
//
// [Ja] CommunityRepository は sqlc 生成のクエリ経由でコミュニティを読み書きします。
type CommunityRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewCommunityRepository creates a CommunityRepository that reads through the
// database's read pool and writes through its write pool.
//
// [Ja] NewCommunityRepository は、データベースの読み取り用プールで読み、書き込み用
// プールで書く CommunityRepository を生成します。
func NewCommunityRepository(db *database.DB) *CommunityRepository {
	return &CommunityRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new CommunityRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい CommunityRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *CommunityRepository) WithTx(tx *sql.Tx) *CommunityRepository {
	q := r.writer.WithTx(tx)
	return &CommunityRepository{reader: q, writer: q}
}

// Find returns the community this instance hosts, or (nil, nil) when the
// instance has not been set up yet. It takes no identifier because the table
// holds at most the single row id 1 (ADR 0006).
//
// Absence is a normal outcome rather than an error: nothing in the application
// creates the row yet, so a migrated database answers this way, and a caller
// renders what it can without the name.
//
// [Ja] Find はこのインスタンスが運営するコミュニティを返し、インスタンスがまだ
// 立ち上げられていない場合は (nil, nil) を返します。テーブルが持ちうるのは id 1 の
// 1 行だけ (ADR 0006) のため、識別子を取りません。
//
// 未存在はエラーではなく正常な結果です。アプリケーションにはまだこの行を作るものが
// 無いため、マイグレーション済みのデータベースはこの答えを返し、呼び出し側は名前が
// 無いなりに描画できるものを描画します。
func (r *CommunityRepository) Find(ctx context.Context) (*model.Community, error) {
	row, err := r.reader.GetCommunity(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.Community row into a model.Community, casting the
// raw id into the typed CommunityID and the stored timestamps back into
// time.Time at the repository boundary.
//
// [Ja] toModel は query.Community を model.Community に変換し、リポジトリの境界で生の id を
// 型付きの CommunityID に、保存書式の時刻を time.Time にキャストします。
func (r *CommunityRepository) toModel(row query.Community) *model.Community {
	return &model.Community{
		ID:        model.CommunityID(row.ID),
		Name:      row.Name,
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
