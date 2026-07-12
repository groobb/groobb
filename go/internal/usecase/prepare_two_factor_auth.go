package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// PrepareTwoFactorAuthUsecase prepares the two-factor authentication setup step by
// resolving the enrollment secret to show as a QR code: it reuses the user's
// in-progress (not-yet-enabled) enrollment if one exists, and otherwise generates a
// fresh secret and persists a not-yet-enabled row. Reusing the existing secret
// keeps a QR the user may have already scanned valid across revisits, and enabling
// (a later step) is what turns the row into an active credential. It takes no
// validator because the input is just the signed-in user id, and no transaction
// because it makes at most one persistence call.
//
// [Ja] PrepareTwoFactorAuthUsecase は 2 段階認証の設定ステップを準備し、QR コードとして
// 表示する登録用 secret を解決します。ユーザーに登録中 (未有効化) の設定があれば再利用し、
// なければ新しい secret を生成して未有効化の行を永続化します。既存 secret の再利用により、
// ユーザーが既にスキャンした QR が再訪しても有効なままになります。行をアクティブな資格情報に
// するのは有効化 (後続ステップ) です。入力はサインイン済みユーザー id だけのため validator は
// 取らず、永続化呼び出しは高々 1 回のためトランザクションも取りません。
type PrepareTwoFactorAuthUsecase struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewPrepareTwoFactorAuthUsecase builds a PrepareTwoFactorAuthUsecase from the 2FA
// repository.
//
// [Ja] NewPrepareTwoFactorAuthUsecase は 2FA リポジトリから PrepareTwoFactorAuthUsecase を
// 構築します。
func NewPrepareTwoFactorAuthUsecase(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *PrepareTwoFactorAuthUsecase {
	return &PrepareTwoFactorAuthUsecase{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// PrepareTwoFactorAuthInput is the input to Execute. UserID is the signed-in user
// setting up 2FA (established by the session).
//
// [Ja] PrepareTwoFactorAuthInput は Execute の入力です。UserID は 2FA を設定するサインイン
// 済みユーザー (セッションで確定する) です。
type PrepareTwoFactorAuthInput struct {
	UserID model.UserID
}

// PrepareTwoFactorAuthOutput carries the enrollment secret to render as a QR code
// and manual-entry key. AlreadyEnabled is true when the user's 2FA is already
// active, in which case Secret is empty and the caller should not re-enroll (it
// redirects to the settings hub instead of showing the setup form).
//
// [Ja] PrepareTwoFactorAuthOutput は QR コードと手動入力キーとして描画する登録用 secret を
// 運びます。AlreadyEnabled はユーザーの 2FA が既にアクティブなとき true で、その場合 Secret は
// 空であり、呼び出し側は再登録すべきではありません (設定フォームを表示せず設定ハブへ
// リダイレクトします)。
type PrepareTwoFactorAuthOutput struct {
	Secret         string
	AlreadyEnabled bool
}

// Execute resolves the enrollment secret. It reports AlreadyEnabled when the user
// already has active 2FA, reuses an in-progress enrollment's secret when one
// exists, and otherwise generates a secret and inserts a not-yet-enabled row. The
// insert is ON CONFLICT (user_id) DO NOTHING: if a concurrent first-time request
// inserted the enrollment first, Create returns nil, so we re-resolve and reuse
// that row instead of failing on the unique constraint. The generate-then-insert
// is a single persistence call preceded by one cheap step, so it stays in Execute
// and needs no transaction.
//
// [Ja] Execute は登録用 secret を解決します。ユーザーが既にアクティブな 2FA を持つときは
// AlreadyEnabled を報告し、登録中の設定があればその secret を再利用し、なければ secret を
// 生成して未有効化の行を挿入します。挿入は ON CONFLICT (user_id) DO NOTHING のため、同時の
// 初回リクエストが先に登録を挿入した場合は Create が nil を返すので、unique 制約で失敗せず
// その行を再解決して再利用します。生成してから挿入する処理は軽い前処理 1 つを伴う単一の
// 永続化呼び出しのため、Execute 内に置き、トランザクションは不要です。
func (uc *PrepareTwoFactorAuthUsecase) Execute(ctx context.Context, input PrepareTwoFactorAuthInput) (*PrepareTwoFactorAuthOutput, error) {
	out, err := uc.resolveExisting(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return out, nil
	}

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		return nil, fmt.Errorf("TOTP secret の生成に失敗: %w", err)
	}

	created, err := uc.userTwoFactorAuthRepo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: input.UserID,
		Secret: secret,
	})
	if err != nil {
		return nil, fmt.Errorf("2 段階認証設定の作成に失敗: %w", err)
	}
	if created == nil {
		// A concurrent request inserted the enrollment first (ON CONFLICT DO
		// NOTHING returned no row). Re-resolve and reuse that row's secret so the
		// two requests converge on the same enrollment.
		//
		// [Ja] 同時のリクエストが先に登録を挿入した (ON CONFLICT DO NOTHING が行を
		// 返さなかった)。その行を再解決して secret を再利用し、2 つのリクエストが同じ
		// 登録に収束するようにする。
		out, err := uc.resolveExisting(ctx, input.UserID)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return nil, fmt.Errorf("2 段階認証設定の作成が競合したが再取得で見つからない: user_id=%s", input.UserID)
		}
		return out, nil
	}

	return &PrepareTwoFactorAuthOutput{Secret: created.Secret}, nil
}

// resolveExisting returns the output for an already-present setting: AlreadyEnabled
// when 2FA is active, or the reusable enrollment secret when setup is still in
// progress. It returns (nil, nil) when the user has no setting yet, signaling the
// caller to create one. Execute calls it both up front and again after an insert
// conflict, so keeping it in one place avoids duplicating the enabled/in-progress
// branching.
//
// [Ja] resolveExisting は既に存在する設定に対する出力を返します。2FA が有効なら
// AlreadyEnabled を、設定がまだ登録中なら再利用可能な登録用 secret を返します。ユーザーに
// 設定がまだ無いときは (nil, nil) を返し、呼び出し側に作成を促します。Execute は最初と、
// 挿入競合の後の 2 回これを呼ぶため、1 箇所にまとめることで有効/登録中の分岐の重複を
// 避けます。
func (uc *PrepareTwoFactorAuthUsecase) resolveExisting(ctx context.Context, userID model.UserID) (*PrepareTwoFactorAuthOutput, error) {
	existing, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("2 段階認証設定の取得に失敗: %w", err)
	}
	if existing == nil {
		return nil, nil
	}
	if existing.Enabled {
		return &PrepareTwoFactorAuthOutput{AlreadyEnabled: true}, nil
	}
	return &PrepareTwoFactorAuthOutput{Secret: existing.Secret}, nil
}
