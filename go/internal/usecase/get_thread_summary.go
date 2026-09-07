package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetThreadSummaryInput addresses the thread to read by its id, the way
// GetThreadInput does: a thread is named by id because its title can be edited.
//
// [Ja] GetThreadSummaryInput は読み取るスレッドを、GetThreadInput と同じく id で指定
// します。スレッドを id で名指すのは、そのタイトルが編集されうるためです。
type GetThreadSummaryInput struct {
	ID model.ThreadID
}

// GetThreadSummaryOutput is the thread itself, without the conversation written
// in it.
//
// [Ja] GetThreadSummaryOutput はスレッド自身であり、そこに書かれた会話は伴いません。
type GetThreadSummaryOutput struct {
	Thread *model.Thread
}

// GetThreadSummaryUsecase reads the thread a page names without reading what is
// written in it. It is a read UseCase: it only calls the lookup methods of its
// repository, so it needs neither a validator nor a transaction.
//
// It stands beside GetThreadUsecase, which reads a thread's page: the thread,
// where it sits, and every post in it. A thread is served whole (ADR 0009), so
// that read is bounded only by the post cap, and a page that names a thread
// without showing the conversation would pay for a thousand posts and draw none
// of them. The page a refused reply comes back on is such a page, and the
// refusal it most often answers — a thread that has filled up — is the one where
// that read is heaviest.
//
// [Ja] GetThreadSummaryUsecase は、ページが名指すスレッドを、そこに書かれたものを読まずに
// 読みます。読み取り UseCase であり、リポジトリの取得系メソッドしか呼ばないため、
// validator もトランザクションも必要としません。
//
// スレッドのページ (スレッド・その在り処・その中のすべての投稿) を読む GetThreadUsecase の
// 傍らに置きます。スレッドは丸ごと配信されるため (ADR 0009)、そちらの読み取りの上限は
// 投稿数の上限だけであり、会話を見せずにスレッドを名指すページは、1000 件分を支払って
// 1 件も描かないことになります。拒否された返信が戻ってくるページはそのようなページであり、
// そこが最も多く応答する拒否 (満杯になったスレッド) は、その読み取りが最も重くなる場合
// でもあります。
type GetThreadSummaryUsecase struct {
	threadRepo *repository.ThreadRepository
}

// NewGetThreadSummaryUsecase builds a GetThreadSummaryUsecase over the thread
// repository.
//
// [Ja] NewGetThreadSummaryUsecase はスレッドのリポジトリから GetThreadSummaryUsecase を
// 構築します。
func NewGetThreadSummaryUsecase(threadRepo *repository.ThreadRepository) *GetThreadSummaryUsecase {
	return &GetThreadSummaryUsecase{threadRepo: threadRepo}
}

// Execute resolves the id to the thread it names.
//
// An id naming no thread is reported as an AppError carrying
// AppErrCodeResourceNotFound, as GetThreadUsecase reports it, so that a handler
// answering either read tells the same 404 either way. It is the known outcome
// of an address left behind by a deleted thread rather than a failure, so it is
// not logged as an error here.
//
// [Ja] Execute は id を、それが名指すスレッドへ解決します。
//
// どのスレッドも指さない id は、GetThreadUsecase がそうするのと同じく
// AppErrCodeResourceNotFound を持つ AppError として報告します。どちらの読み取りに応答する
// ハンドラーも、同じ 404 を伝えるようにするためです。これは削除されたスレッドの残した
// アドレスの既知の結果であって失敗ではないため、ここではエラーとしてログに残しません。
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

	return &GetThreadSummaryOutput{Thread: thread}, nil
}
