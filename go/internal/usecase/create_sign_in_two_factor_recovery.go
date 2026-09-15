package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateSignInTwoFactorRecoveryUsecaseはサインイン時のリカバリーコードチャレンジを
// 統括します。送信されたコードが保留中ユーザーの保存済みリカバリーコードの1つであることを
// 検証し、成功時にそのコードの消費とセッションの発行を1トランザクションで行います。TOTP
// チャレンジ (コード検証は純粋な読み取りで、ハンドラーが別途セッションを発行する) と違い、
// リカバリーコードは1回使い切りです。コードの消費 (保存済みの集合からの削除) とセッションの
// 作成はアトミックである必要があり、使い切ったコードだけでセッションが無い状態も、セッション
// だけでコードが消費されていない状態も残さないようにします。セッション作成は本トランザクション
// 内で走る必要があるため、共有のCreateSessionUsecaseではなくここに置きます。
type CreateSignInTwoFactorRecoveryUsecase struct {
	writer                *sql.DB
	validator             *validator.SignInTwoFactorRecoveryCreateValidator
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
	userSessionRepo       *repository.UserSessionRepository
}

// NewCreateSignInTwoFactorRecoveryUsecaseは書き込み用プール・validator・永続化に使う
// リポジトリからCreateSignInTwoFactorRecoveryUsecaseを構築します。
func NewCreateSignInTwoFactorRecoveryUsecase(
	writer *sql.DB,
	validator *validator.SignInTwoFactorRecoveryCreateValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userSessionRepo *repository.UserSessionRepository,
) *CreateSignInTwoFactorRecoveryUsecase {
	return &CreateSignInTwoFactorRecoveryUsecase{
		writer:                writer,
		validator:             validator,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
		userSessionRepo:       userSessionRepo,
	}
}

// CreateSignInTwoFactorRecoveryInputはExecuteの入力です。UserIDは2段階認証
// Cookieから解決した保留中ユーザー、Codeは送信されたリカバリーコード、IPAddress /
// UserAgentは監査のためセッションを確立した場所を記録します。
type CreateSignInTwoFactorRecoveryInput struct {
	UserID    model.UserID
	Code      string
	IPAddress string
	UserAgent string
}

// CreateSignInTwoFactorRecoveryOutputは不透明なセッショントークンを運び、
// ハンドラーがそれをセッションCookieに格納できるようにします。
type CreateSignInTwoFactorRecoveryOutput struct {
	Token string
}

// Executeはリカバリーコードを検証してから、それを消費しつつセッションを発行します。
// バリデーションを先に走らせるため、未入力・形式不正・未知・失われたチャレンジのコードは行に
// 触れずvalidatorのエラー (*model.ValidationError、またはシステム障害なら素のerror) を
// 返します。残りのコードとセッショントークンはトランザクションの前に計算し (データベースに
// 依存しないため)、トランザクションを永続化のみに保ちます。
func (uc *CreateSignInTwoFactorRecoveryUsecase) Execute(ctx context.Context, input CreateSignInTwoFactorRecoveryInput) (*CreateSignInTwoFactorRecoveryOutput, error) {
	twoFactorAuth, err := uc.validator.Validate(ctx, validator.SignInTwoFactorRecoveryCreateValidatorInput{
		UserID: input.UserID,
		Code:   input.Code,
	})
	if err != nil {
		return nil, err
	}

	remainingCodes := removeRecoveryCode(twoFactorAuth.RecoveryCodes, input.Code)

	token, err := auth.GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("セッショントークンの生成に失敗: %w", err)
	}

	if err := uc.consumeAndCreateSession(ctx, input, remainingCodes, token); err != nil {
		return nil, err
	}

	return &CreateSignInTwoFactorRecoveryOutput{Token: token}, nil
}

// consumeAndCreateSessionは残りのリカバリーコードの書き込みとセッションの作成を
// 1トランザクションで行い、使用したコードと新しいセッションが両方成るか、どちらも成らないか
// にします。この2つの永続化ステップがあるため、本処理をExecute (純粋なオーケストレーションに
// 徹する) から切り出しています。
func (uc *CreateSignInTwoFactorRecoveryUsecase) consumeAndCreateSession(ctx context.Context, input CreateSignInTwoFactorRecoveryInput, remainingCodes []string, token string) error {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userTwoFactorAuthRepo := uc.userTwoFactorAuthRepo.WithTx(tx)
	userSessionRepo := uc.userSessionRepo.WithTx(tx)

	if err := userTwoFactorAuthRepo.UpdateRecoveryCodes(ctx, input.UserID, remainingCodes); err != nil {
		return fmt.Errorf("リカバリーコードの更新に失敗: %w", err)
	}
	if _, err := userSessionRepo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    input.UserID,
		Token:     token,
		IPAddress: input.IPAddress,
		UserAgent: input.UserAgent,
	}); err != nil {
		return fmt.Errorf("セッションの作成に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}

// removeRecoveryCodeは保存済みコードからusedの最初の1つを除いたものを返し、
// リカバリーコードを使用済み (1回使い切り) にします。validatorがusedの存在を既に確認
// しているため、ちょうど1つが取り除かれます。全コードを消費した結果は非nilの空スライスで、
// リポジトリがJSONの空配列へエンコードしてTEXT列へ保存します。
func removeRecoveryCode(codes []string, used string) []string {
	remaining := make([]string, 0, len(codes))
	removed := false
	for _, c := range codes {
		if !removed && c == used {
			removed = true
			continue
		}
		remaining = append(remaining, c)
	}
	return remaining
}
