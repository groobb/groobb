package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetThreadSummaryInputは読み取るスレッドを、GetThreadInputと同じくidで指定
// します。スレッドをidで名指すのは、そのタイトルが編集されうるためです。
type GetThreadSummaryInput struct {
	ID model.ThreadID
}

// GetThreadSummaryOutputはスレッド自身であり、そこに書かれた会話は伴いません。
type GetThreadSummaryOutput struct {
	Thread *model.Thread
}

// GetThreadSummaryUsecaseは、ページが名指すスレッドを、そこに書かれたものを読まずに
// 読みます。読み取りUseCaseであり、リポジトリの取得系メソッドしか呼ばないため、
// validatorもトランザクションも必要としません。
//
// スレッドのページ (スレッド・その在り処・その中のすべての投稿) を読むGetThreadUsecaseの
// 傍らに置きます。スレッドは丸ごと配信されるため (ADR 0009)、そちらの読み取りの上限は
// 投稿数の上限だけであり、会話を見せずにスレッドを名指すページは、1000件分を支払って
// 1件も描かないことになります。拒否された返信が戻ってくるページはそのようなページであり、
// そこが最も多く応答する拒否 (満杯になったスレッド) は、その読み取りが最も重くなる場合
// でもあります。
type GetThreadSummaryUsecase struct {
	threadRepo *repository.ThreadRepository
}

// NewGetThreadSummaryUsecaseはスレッドのリポジトリからGetThreadSummaryUsecaseを
// 構築します。
func NewGetThreadSummaryUsecase(threadRepo *repository.ThreadRepository) *GetThreadSummaryUsecase {
	return &GetThreadSummaryUsecase{threadRepo: threadRepo}
}

// Executeはidを、それが名指すスレッドへ解決します。
//
// どのスレッドも指さないidは、GetThreadUsecaseがそうするのと同じく
// AppErrCodeResourceNotFoundを持つAppErrorとして報告します。どちらの読み取りに応答する
// ハンドラーも、同じ404を伝えるようにするためです。これは削除されたスレッドの残した
// アドレスの既知の結果であって失敗ではないため、ここではエラーとしてログに残しません。
//
// 管理者が非公開にしたスレッドは、GetThreadUsecaseがそうするのと同じく
// AppErrCodeResourceUnpublishedとして報告します。この読み取りが配信するのは拒否された返信が
// 戻ってくるページであり、何も示さなくなったスレッドはもう書き込む先のページではないため、
// 拒否には、それが書かれたフォームではなくスレッドの取り下げで応答します。
func (uc *GetThreadSummaryUsecase) Execute(ctx context.Context, input GetThreadSummaryInput) (*GetThreadSummaryOutput, error) {
	thread, err := uc.threadRepo.FindByID(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("スレッドの取得に失敗: %w", err)
	}
	if thread == nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("スレッドが見つからない: id=%s", input.ID),
			Metadata: map[string]string{"thread_id": input.ID.String()},
		}
	}
	if thread.UnpublishedAt != nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceUnpublished,
			UserMsg:  i18n.T(ctx, "error_unpublished_message"),
			Internal: fmt.Errorf("非公開のスレッドの読み取り: id=%s", input.ID),
			Metadata: map[string]string{"thread_id": input.ID.String()},
		}
	}

	return &GetThreadSummaryOutput{Thread: thread}, nil
}
