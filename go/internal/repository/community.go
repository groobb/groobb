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

// CommunityRepositoryはsqlc生成のクエリ経由でコミュニティを読み書きします。
type CommunityRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewCommunityRepositoryは、データベースの読み取り用プールで読み、書き込み用
// プールで書くCommunityRepositoryを生成します。
func NewCommunityRepository(db *database.DB) *CommunityRepository {
	return &CommunityRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいCommunityRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *CommunityRepository) WithTx(tx *sql.Tx) *CommunityRepository {
	q := r.writer.WithTx(tx)
	return &CommunityRepository{reader: q, writer: q}
}

// Findはこのインスタンスが運営するコミュニティを返し、インスタンスがまだ
// 立ち上げられていない場合は (nil, nil) を返します。テーブルが持ちうるのはid 1の
// 1行だけ (ADR 0006) のため、識別子を取りません。
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

// toModelはquery.Communityをmodel.Communityに変換し、リポジトリの境界で生のidを
// 型付きのCommunityIDに、保存書式の時刻をtime.Timeにキャストします。
func (r *CommunityRepository) toModel(row query.Community) *model.Community {
	return &model.Community{
		ID:        model.CommunityID(row.ID),
		Name:      row.Name,
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
