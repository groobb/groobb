package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/repository"
)

// DeleteSessionUsecaseはセッション行を削除してユーザーをサインアウトさせ、Cookieが
// 再生されてもトークンがユーザーに解決しないようにします。セッションCookieの消去は
// ハンドラーの別ステップです。validatorは持たず (入力はリクエスト自身のセッション
// トークンでありユーザーのフォーム入力ではない)、トランザクションも不要 (単一の削除) の
// ため、Executeが処理全体を保持します。
type DeleteSessionUsecase struct {
	userSessionRepo *repository.UserSessionRepository
}

// NewDeleteSessionUsecaseはセッションリポジトリからDeleteSessionUsecaseを
// 構築します。
func NewDeleteSessionUsecase(userSessionRepo *repository.UserSessionRepository) *DeleteSessionUsecase {
	return &DeleteSessionUsecase{userSessionRepo: userSessionRepo}
}

// Executeはtokenが指すセッションを削除します。tokenが空のときは何もしません。
// 未サインインでのサインアウトはエラーではなく、DeleteByTokenが1行も削除しなくても
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
