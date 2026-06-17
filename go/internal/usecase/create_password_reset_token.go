package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreatePasswordResetTokenUsecase orchestrates a password reset request: it
// validates the email, issues a one-time reset token (storing only its hash),
// and enqueues the mail carrying the reset link. It is enumeration-safe: an
// unknown email produces no token and no mail but the same outcome to the caller,
// so the response never reveals whether an account exists. The token itself is
// spent by the separate password-update flow.
//
// [Ja] CreatePasswordResetTokenUsecase はパスワードリセット申請を統括します。email を
// 検証し、使い捨てのリセットトークンを発行し (ハッシュのみ保存)、リセットリンクを運ぶ
// メールを投入します。列挙攻撃に対して安全です。未知の email ではトークンもメールも作らず、
// 呼び出し側への結果は同じになるため、レスポンスはアカウントの存在有無を決して明かしません。
// トークン自体は別のパスワード更新フローで消費されます。
type CreatePasswordResetTokenUsecase struct {
	db                     *pgxpool.Pool
	passwordResetValidator *validator.PasswordResetCreateValidator
	userRepo               *repository.UserRepository
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
	dispatcher             *dispatcher.Dispatcher
	cfg                    *config.Config
}

// NewCreatePasswordResetTokenUsecase builds a CreatePasswordResetTokenUsecase
// from the pool, validator, repositories, dispatcher, and config.
//
// [Ja] NewCreatePasswordResetTokenUsecase はプール・validator・リポジトリ・dispatcher・
// config から CreatePasswordResetTokenUsecase を構築します。
func NewCreatePasswordResetTokenUsecase(
	db *pgxpool.Pool,
	passwordResetValidator *validator.PasswordResetCreateValidator,
	userRepo *repository.UserRepository,
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	dispatcher *dispatcher.Dispatcher,
	cfg *config.Config,
) *CreatePasswordResetTokenUsecase {
	return &CreatePasswordResetTokenUsecase{
		db:                     db,
		passwordResetValidator: passwordResetValidator,
		userRepo:               userRepo,
		passwordResetTokenRepo: passwordResetTokenRepo,
		dispatcher:             dispatcher,
		cfg:                    cfg,
	}
}

// CreatePasswordResetTokenInput is the input to Execute. Locale is the request
// locale, carried so the reset mail is rendered in the language the user is
// browsing in.
//
// [Ja] CreatePasswordResetTokenInput は Execute の入力です。Locale はリクエストの
// ロケールで、リセットメールをユーザーが閲覧中の言語で描画するために運びます。
type CreatePasswordResetTokenInput struct {
	Email  string
	Locale string
}

// CreatePasswordResetTokenOutput carries the created reset token, or is nil when
// the email did not match any account (the enumeration-safe no-op path). The
// handler ignores the value and shows the same confirmation either way; the
// output exists so callers and tests can tell the created path from the no-op.
//
// [Ja] CreatePasswordResetTokenOutput は作成されたリセットトークンを運びます。email が
// どのアカウントにも一致しなかったとき (列挙攻撃対策の no-op 経路) は nil です。ハンドラーは
// 値を無視しどちらでも同じ確認を表示します。出力は呼び出し側やテストが作成経路と no-op を
// 区別できるように存在します。
type CreatePasswordResetTokenOutput struct {
	Token *model.PasswordResetToken
}

// Execute validates the email, resolves the account, and—only when it exists—
// issues a fresh reset token and enqueues the mail. Token generation and hashing
// run before the transaction, which is kept to pure persistence. A mail-enqueue
// failure is logged but not returned: the token is already valid, and failing
// here would both strand the user and risk revealing (via an error response) that
// the address belongs to an account.
//
// [Ja] Execute は email を検証し、アカウントを解決し、存在するときに限り新しいリセット
// トークンを発行してメールを投入します。トークンの生成とハッシュ化はトランザクションの前に
// 実行し、トランザクションは純粋な永続化に保ちます。メール投入の失敗はログに記録しますが
// 返しません。トークンは既に有効で、ここで失敗するとユーザーを手詰まりにし、かつ (エラー
// レスポンスを通じて) そのアドレスがアカウントに属することを明かす恐れがあるためです。
func (uc *CreatePasswordResetTokenUsecase) Execute(ctx context.Context, input CreatePasswordResetTokenInput) (*CreatePasswordResetTokenOutput, error) {
	if err := uc.passwordResetValidator.Validate(ctx, validator.PasswordResetCreateValidatorInput{
		Email: input.Email,
	}); err != nil {
		return nil, err
	}

	user, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		// Unknown email: issue nothing and report success with a nil token, so the
		// handler shows the same confirmation as for a real account and the
		// response does not reveal whether the address is registered.
		//
		// [Ja] 未知の email: 何も発行せず、nil のトークンで成功を報告する。これにより
		// ハンドラーは実在アカウントと同じ確認を表示し、レスポンスはそのアドレスが登録
		// 済みかどうかを明かさない。
		return nil, nil
	}

	// Generate the one-time token and its digest before the transaction (logic,
	// not persistence). The plaintext goes into the reset link; only the digest is
	// stored.
	//
	// [Ja] 使い捨てトークンとそのダイジェストをトランザクションの前に生成する (永続化では
	// なくロジック)。平文はリセットリンクに入れ、保存するのはダイジェストだけ。
	rawToken, err := auth.GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("リセットトークンの生成に失敗: %w", err)
	}
	tokenDigest := auth.HashToken(rawToken)
	expiresAt := time.Now().Add(model.PasswordResetTokenExpirationDuration)

	token, err := uc.createToken(ctx, user.ID, tokenDigest, expiresAt)
	if err != nil {
		return nil, err
	}

	// Build the reset link and enqueue the mail. A send-enqueue failure is logged
	// and swallowed: the token is valid regardless, and returning an error only on
	// the existing-account path would leak account existence.
	//
	// [Ja] リセットリンクを組み立ててメールを投入する。投入の失敗はログに記録して握り潰す。
	// トークンはいずれにせよ有効で、実在アカウントの経路でだけエラーを返すとアカウントの
	// 存在が漏れるため。
	resetURL := fmt.Sprintf("%s/password/edit?token=%s", uc.cfg.AppURL, rawToken)
	if err := uc.dispatcher.EnqueuePasswordReset(ctx, user.Email, resetURL, input.Locale); err != nil {
		slog.ErrorContext(ctx, "パスワードリセットメールのジョブ投入に失敗", "error", err, "user_id", user.ID.String())
	}

	return &CreatePasswordResetTokenOutput{Token: token}, nil
}

// createToken replaces the user's outstanding unused tokens with a freshly
// issued one in a single transaction, so a new request invalidates any earlier
// link and the user never accumulates multiple live reset tokens. The digest and
// expiry are computed by Execute beforehand, keeping the transaction to pure
// persistence.
//
// [Ja] createToken はユーザーの未使用の既存トークンを、新しく発行した 1 つに 1 つの
// トランザクションで置き換えます。新しい申請で以前のリンクを無効化し、ユーザーが複数の
// 有効なリセットトークンを溜め込まないようにします。ダイジェストと有効期限は事前に
// Execute が計算済みで、トランザクションを純粋な永続化に保ちます。
func (uc *CreatePasswordResetTokenUsecase) createToken(ctx context.Context, userID model.UserID, tokenDigest string, expiresAt time.Time) (*model.PasswordResetToken, error) {
	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tokenRepo := uc.passwordResetTokenRepo.WithTx(tx)

	if err := tokenRepo.DeleteUnusedByUserID(ctx, userID); err != nil {
		return nil, fmt.Errorf("既存リセットトークンの削除に失敗: %w", err)
	}

	token, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: tokenDigest,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("リセットトークンの作成に失敗: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return token, nil
}
