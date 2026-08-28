package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetBoardInput addresses the board to read by the slug /b/{slug} carries.
//
// [Ja] GetBoardInput は読み取る掲示板を、/b/{slug} が運ぶ slug で指定します。
type GetBoardInput struct {
	Slug string
}

// GetBoardOutput is the board the slug resolved to, together with the category
// that lists it. The category comes along because /b/{slug} names a board
// without naming where it sits, so the page has to say it: the breadcrumb is the
// only place a visitor learns which part of the community they are in.
//
// Category is nil for a board sitting in none, which is a normal state rather
// than a gap (ADR 0011). The page then has no place above the board to name.
//
// The threads it holds are read separately by GetBoardThreadsUsecase, because
// /b/{slug} decides whether the page is going to be rendered at all from the
// board alone.
//
// [Ja] GetBoardOutput は slug が解決した掲示板と、それを並べるカテゴリーです。
// カテゴリーが伴うのは、/b/{slug} が掲示板をその在り処を言わずに名指しするため、
// ページ側がそれを述べる必要があるからです。訪問者がコミュニティのどの部分にいるのかを
// 知る手立ては、パンくずだけです。
//
// どのカテゴリーにも属さない掲示板では Category が nil になります。これは欠落ではなく
// 正常な状態であり (ADR 0011)、その場合ページには掲示板の上位として名指す場所が
// ありません。
//
// そこに立つスレッドは GetBoardThreadsUsecase が別に読みます。/b/{slug} は、そもそも
// ページを描画するかどうかを掲示板だけで決めるためです。
type GetBoardOutput struct {
	Board    *model.Board
	Category *model.Category
}

// GetBoardUsecase resolves a slug to one board and the category that lists it.
// It is a read UseCase: it only calls the lookup methods of its repositories, so
// it needs neither a validator nor a transaction.
//
// [Ja] GetBoardUsecase は slug を掲示板 1 つと、それを並べるカテゴリーへ解決します。
// 読み取り UseCase であり、リポジトリの取得系メソッドしか呼ばないため、validator も
// トランザクションも必要としません。
type GetBoardUsecase struct {
	boardRepo    *repository.BoardRepository
	categoryRepo *repository.CategoryRepository
}

// NewGetBoardUsecase builds a GetBoardUsecase over the board and category
// repositories.
//
// [Ja] NewGetBoardUsecase は掲示板とカテゴリーの各リポジトリから GetBoardUsecase を
// 構築します。
func NewGetBoardUsecase(boardRepo *repository.BoardRepository, categoryRepo *repository.CategoryRepository) *GetBoardUsecase {
	return &GetBoardUsecase{boardRepo: boardRepo, categoryRepo: categoryRepo}
}

// Execute resolves the slug to a board and reads the category that lists it.
//
// A slug naming no board is reported as an AppError carrying
// AppErrCodeResourceNotFound, which is what lets the handler answer 404 with the
// shared not-found page. It is a known outcome of a URL that was typed, guessed,
// or left behind by a deleted board rather than a failure, so it is not logged
// as an error here.
//
// A board naming a category that cannot be read back is a failure instead.
// Deleting a category clears the column rather than leaving it pointing at a row
// that is gone, so a board still naming one names a category that is there.
// Reporting it as a missing page would tell a crawler to drop a board that is
// still there.
//
// [Ja] Execute は slug を掲示板へ解決し、それを並べるカテゴリーを読みます。
//
// どの掲示板も指さない slug は AppErrCodeResourceNotFound を持つ AppError として報告し、
// ハンドラーが共通の not-found ページで 404 を返せるようにします。これは手で打たれた・
// 推測された・削除された掲示板の残した URL の既知の結果であって失敗ではないため、
// ここではエラーとしてログに残しません。
//
// 一方、カテゴリーを名指しているのにそれを読み戻せない場合は失敗です。カテゴリーの削除は
// 列を、消えた行を指したままにするのではなく空にするため、まだ名指している掲示板が指す
// カテゴリーは存在します。これをページの不在として報告すれば、まだ存在する掲示板を落とす
// ようクローラーに伝えてしまいます。
func (uc *GetBoardUsecase) Execute(ctx context.Context, input GetBoardInput) (*GetBoardOutput, error) {
	board, err := uc.boardRepo.FindBySlug(ctx, input.Slug)
	if err != nil {
		return nil, fmt.Errorf("掲示板の取得に失敗: %w", err)
	}
	if board == nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("掲示板が見つからない: slug=%s", input.Slug),
			Metadata: map[string]string{"board_slug": input.Slug},
		}
	}

	if board.CategoryID == nil {
		return &GetBoardOutput{Board: board}, nil
	}

	category, err := uc.categoryRepo.FindByID(ctx, *board.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("掲示板のカテゴリーの取得に失敗: %w", err)
	}
	if category == nil {
		return nil, fmt.Errorf("掲示板のカテゴリーが見つからない: board_id=%s category_id=%s", board.ID, *board.CategoryID)
	}

	return &GetBoardOutput{Board: board, Category: category}, nil
}
