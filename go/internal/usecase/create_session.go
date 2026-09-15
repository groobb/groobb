package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// CreateSessionUsecaseはユーザーのサインイン済みセッションを作成します。不透明な
// セッショントークンを生成し、user_sessions行を永続化します。入力は既に信頼できる
// データ (前段で解決されたUserIDと、リクエストのIP / User-Agent) であってユーザーの
// フォーム入力ではないため、validatorは取りません。呼び出し側 (アカウント作成・
// サインインのハンドラー) は返したトークンをセッションCookieに設定します。
type CreateSessionUsecase struct {
	userSessionRepo *repository.UserSessionRepository
}

// NewCreateSessionUsecaseはセッションリポジトリからCreateSessionUsecaseを
// 構築します。
func NewCreateSessionUsecase(userSessionRepo *repository.UserSessionRepository) *CreateSessionUsecase {
	return &CreateSessionUsecase{userSessionRepo: userSessionRepo}
}

// CreateSessionInputはExecuteの入力です。UserIDは誰をサインインさせるかを
// 識別し、IPAddress / UserAgentは監査のためセッションを確立した場所を記録します。
type CreateSessionInput struct {
	UserID    model.UserID
	IPAddress string
	UserAgent string
}

// CreateSessionOutputは不透明なセッショントークンを運び、ハンドラーがそれを
// セッションCookieに格納できるようにします。
type CreateSessionOutput struct {
	Token string
}

// Executeはセッショントークンを生成しセッションを永続化します。永続化が1回で
// 前処理 (トークン生成) も軽いため、プライベート関数ではなくExecute内に置き、
// トランザクションも不要です。
func (uc *CreateSessionUsecase) Execute(ctx context.Context, input CreateSessionInput) (*CreateSessionOutput, error) {
	token, err := auth.GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("セッショントークンの生成に失敗: %w", err)
	}

	if _, err := uc.userSessionRepo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    input.UserID,
		Token:     token,
		IPAddress: input.IPAddress,
		UserAgent: input.UserAgent,
	}); err != nil {
		return nil, fmt.Errorf("セッションの作成に失敗: %w", err)
	}

	return &CreateSessionOutput{Token: token}, nil
}
