package usecase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateSignInTwoFactorRecoveryUsecase orchestrates the sign-in recovery-code
// challenge: it validates that the submitted code is one of the pending user's
// stored recovery codes and, on success, consumes that code and issues the session
// in a single transaction. Unlike the TOTP challenge (where verifying a code is a
// pure read and the handler issues the session separately), a recovery code is
// one-time: consuming it (removing it from the stored set) and creating the session
// must be atomic, so neither a spent code without a session nor a session without a
// spent code is ever left behind. Session creation lives here rather than in the
// shared CreateSessionUsecase because it must run inside this transaction.
//
// [Ja] CreateSignInTwoFactorRecoveryUsecase はサインイン時のリカバリーコードチャレンジを
// 統括します。送信されたコードが保留中ユーザーの保存済みリカバリーコードの 1 つであることを
// 検証し、成功時にそのコードの消費とセッションの発行を 1 トランザクションで行います。TOTP
// チャレンジ (コード検証は純粋な読み取りで、ハンドラーが別途セッションを発行する) と違い、
// リカバリーコードは 1 回使い切りです。コードの消費 (保存済みの集合からの削除) とセッションの
// 作成はアトミックである必要があり、使い切ったコードだけでセッションが無い状態も、セッション
// だけでコードが消費されていない状態も残さないようにします。セッション作成は本トランザクション
// 内で走る必要があるため、共有の CreateSessionUsecase ではなくここに置きます。
type CreateSignInTwoFactorRecoveryUsecase struct {
	db                    *pgxpool.Pool
	validator             *validator.SignInTwoFactorRecoveryCreateValidator
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
	userSessionRepo       *repository.UserSessionRepository
}

// NewCreateSignInTwoFactorRecoveryUsecase builds a
// CreateSignInTwoFactorRecoveryUsecase from the pool, its validator, and the
// repositories it persists through.
//
// [Ja] NewCreateSignInTwoFactorRecoveryUsecase はプール・validator・永続化に使う
// リポジトリから CreateSignInTwoFactorRecoveryUsecase を構築します。
func NewCreateSignInTwoFactorRecoveryUsecase(
	db *pgxpool.Pool,
	validator *validator.SignInTwoFactorRecoveryCreateValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userSessionRepo *repository.UserSessionRepository,
) *CreateSignInTwoFactorRecoveryUsecase {
	return &CreateSignInTwoFactorRecoveryUsecase{
		db:                    db,
		validator:             validator,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
		userSessionRepo:       userSessionRepo,
	}
}

// CreateSignInTwoFactorRecoveryInput is the input to Execute. UserID is the pending
// user resolved from the two-factor cookie, Code is the submitted recovery code,
// and IPAddress / UserAgent record where the session was established for audit.
//
// [Ja] CreateSignInTwoFactorRecoveryInput は Execute の入力です。UserID は 2 段階認証
// Cookie から解決した保留中ユーザー、Code は送信されたリカバリーコード、IPAddress /
// UserAgent は監査のためセッションを確立した場所を記録します。
type CreateSignInTwoFactorRecoveryInput struct {
	UserID    model.UserID
	Code      string
	IPAddress string
	UserAgent string
}

// CreateSignInTwoFactorRecoveryOutput carries the opaque session token so the
// handler can store it in the session cookie.
//
// [Ja] CreateSignInTwoFactorRecoveryOutput は不透明なセッショントークンを運び、
// ハンドラーがそれをセッション Cookie に格納できるようにします。
type CreateSignInTwoFactorRecoveryOutput struct {
	Token string
}

// Execute validates the recovery code and then consumes it while issuing the
// session. Validation runs first, so a missing, malformed, unknown, or
// gone-challenge code returns the validator's error (a *model.ValidationError, or a
// plain error for a system failure) without touching any row. The remaining codes
// and the session token are computed before the transaction (they are pure of the
// database), keeping the transaction to persistence only.
//
// [Ja] Execute はリカバリーコードを検証してから、それを消費しつつセッションを発行します。
// バリデーションを先に走らせるため、未入力・形式不正・未知・失われたチャレンジのコードは行に
// 触れず validator のエラー (*model.ValidationError、またはシステム障害なら素の error) を
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

// consumeAndCreateSession writes the remaining recovery codes and creates the
// session in one transaction, so the used code and the new session commit together
// or not at all. These two persistence steps are why it is split out of Execute
// (which stays pure orchestration).
//
// [Ja] consumeAndCreateSession は残りのリカバリーコードの書き込みとセッションの作成を
// 1 トランザクションで行い、使用したコードと新しいセッションが両方成るか、どちらも成らないか
// にします。この 2 つの永続化ステップがあるため、本処理を Execute (純粋なオーケストレーションに
// 徹する) から切り出しています。
func (uc *CreateSignInTwoFactorRecoveryUsecase) consumeAndCreateSession(ctx context.Context, input CreateSignInTwoFactorRecoveryInput, remainingCodes []string, token string) error {
	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}

// removeRecoveryCode returns the stored codes with the first occurrence of used
// removed, marking a recovery code as spent (one-time use). The validator has
// already confirmed used is present, so exactly one code is dropped; an all-codes
// -consumed result is a non-nil empty slice, which the column (text[] NOT NULL)
// stores as an empty array.
//
// [Ja] removeRecoveryCode は保存済みコードから used の最初の 1 つを除いたものを返し、
// リカバリーコードを使用済み (1 回使い切り) にします。validator が used の存在を既に確認
// しているため、ちょうど 1 つが取り除かれます。全コードを消費した結果は非 nil の空スライスで、
// カラム (text[] NOT NULL) はそれを空配列として保存します。
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
