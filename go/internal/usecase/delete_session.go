package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/repository"
)

// DeleteSessionUsecase signs a user out by deleting their session row, so the
// token can no longer resolve to a user even if the cookie is replayed. Clearing
// the session cookie is the handler's separate step. It has no validator (the
// input is the request's own session token, not user form input) and no
// transaction (a single delete), so Execute holds the whole operation.
//
// [Ja] DeleteSessionUsecase はセッション行を削除してユーザーをサインアウトさせ、Cookie が
// 再生されてもトークンがユーザーに解決しないようにします。セッション Cookie の消去は
// ハンドラーの別ステップです。validator は持たず (入力はリクエスト自身のセッション
// トークンでありユーザーのフォーム入力ではない)、トランザクションも不要 (単一の削除) の
// ため、Execute が処理全体を保持します。
type DeleteSessionUsecase struct {
	userSessionRepo *repository.UserSessionRepository
}

// NewDeleteSessionUsecase builds a DeleteSessionUsecase from the session
// repository.
//
// [Ja] NewDeleteSessionUsecase はセッションリポジトリから DeleteSessionUsecase を
// 構築します。
func NewDeleteSessionUsecase(userSessionRepo *repository.UserSessionRepository) *DeleteSessionUsecase {
	return &DeleteSessionUsecase{userSessionRepo: userSessionRepo}
}

// Execute deletes the session identified by token. An absent token is a no-op:
// signing out when not signed in is not an error, and DeleteByToken deleting no
// row is harmless, so the caller need not special-case it.
//
// [Ja] Execute は token が指すセッションを削除します。token が空のときは何もしません。
// 未サインインでのサインアウトはエラーではなく、DeleteByToken が 1 行も削除しなくても
// 無害なため、呼び出し側で特別扱いする必要はありません。
func (uc *DeleteSessionUsecase) Execute(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	if err := uc.userSessionRepo.DeleteByToken(ctx, token); err != nil {
		return fmt.Errorf("セッションの削除に失敗: %w", err)
	}

	return nil
}
