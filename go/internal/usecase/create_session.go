package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// CreateSessionUsecase creates a signed-in session for a user: it generates an
// opaque session token and persists a user_sessions row. It takes no validator
// because its input is already-trusted data (a UserID resolved by an earlier
// step, plus the request's IP and user agent), not user form input. Callers
// (account creation now, sign-in later) set the session cookie to the returned
// token.
//
// [Ja] CreateSessionUsecase はユーザーのサインイン済みセッションを作成します。不透明な
// セッショントークンを生成し、user_sessions 行を永続化します。入力は既に信頼できる
// データ (前段で解決された UserID と、リクエストの IP / User-Agent) であってユーザーの
// フォーム入力ではないため、validator は取りません。呼び出し側 (現在はアカウント作成、
// 後にサインイン) は返したトークンをセッション Cookie に設定します。
type CreateSessionUsecase struct {
	userSessionRepo *repository.UserSessionRepository
}

// NewCreateSessionUsecase builds a CreateSessionUsecase from the session
// repository.
//
// [Ja] NewCreateSessionUsecase はセッションリポジトリから CreateSessionUsecase を
// 構築します。
func NewCreateSessionUsecase(userSessionRepo *repository.UserSessionRepository) *CreateSessionUsecase {
	return &CreateSessionUsecase{userSessionRepo: userSessionRepo}
}

// CreateSessionInput is the input to Execute. UserID identifies whom to sign in,
// and IPAddress / UserAgent record where the session was established for audit.
//
// [Ja] CreateSessionInput は Execute の入力です。UserID は誰をサインインさせるかを
// 識別し、IPAddress / UserAgent は監査のためセッションを確立した場所を記録します。
type CreateSessionInput struct {
	UserID    model.UserID
	IPAddress string
	UserAgent string
}

// CreateSessionOutput carries the opaque session token so the handler can store
// it in the session cookie.
//
// [Ja] CreateSessionOutput は不透明なセッショントークンを運び、ハンドラーがそれを
// セッション Cookie に格納できるようにします。
type CreateSessionOutput struct {
	Token string
}

// Execute generates a session token and persists the session. It is a single
// persistence call with one cheap pre-step (token generation), so it stays in
// Execute rather than a private helper and needs no transaction.
//
// [Ja] Execute はセッショントークンを生成しセッションを永続化します。永続化が 1 回で
// 前処理 (トークン生成) も軽いため、プライベート関数ではなく Execute 内に置き、
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
