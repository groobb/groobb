// Package repository adapts sqlc-generated queries into domain models. Each
// repository owns one model (model.User <-> UserRepository) and converts query
// rows into that model, keeping the database details out of the upper layers.
//
// [Ja] repository パッケージは sqlc 生成のクエリをドメインモデルに変換します。
// 各リポジトリは 1 つのモデルを担当し (model.User <-> UserRepository)、クエリ結果を
// そのモデルに変換することで、DB の詳細を上位層から隠します。
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// UserRepository reads and writes users through sqlc-generated queries.
//
// [Ja] UserRepository は sqlc 生成のクエリ経由で users を読み書きします。
type UserRepository struct {
	q *query.Queries
}

// NewUserRepository creates a UserRepository backed by the given queries.
//
// [Ja] NewUserRepository は与えられた queries を使う UserRepository を生成します。
func NewUserRepository(q *query.Queries) *UserRepository {
	return &UserRepository{q: q}
}

// WithTx returns a new UserRepository whose queries run inside tx, so a UseCase
// can enlist this repository in its transaction. The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{q: r.q.WithTx(tx)}
}

// FindByID returns the user with the given ID, or (nil, nil) when none exists.
// Absence is a normal lookup outcome, not an error; the caller decides whether
// to treat it as a business-level failure.
//
// [Ja] FindByID は指定 ID のユーザーを返し、存在しない場合は (nil, nil) を返します。
// 未存在は正常なルックアップ結果でありエラーではありません。業務上の失敗として扱うか
// は呼び出し側が判断します。
func (r *UserRepository) FindByID(ctx context.Context, id model.UserID) (*model.User, error) {
	row, err := r.q.GetUserByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindByEmail returns the user with the given email, or (nil, nil) when none
// exists. The email column is citext, so the match ignores letter case.
//
// [Ja] FindByEmail は指定 email のユーザーを返し、存在しない場合は (nil, nil) を
// 返します。email 列は citext のため、照合は大文字小文字を無視します。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindByAtname returns the user with the given atname, or (nil, nil) when none
// exists. The atname column is citext, so the match ignores letter case (the
// same casing rule the atname UNIQUE constraint enforces). Absence is a normal
// lookup outcome used by the uniqueness check, not an error.
//
// [Ja] FindByAtname は指定 atname のユーザーを返し、存在しない場合は (nil, nil) を
// 返します。atname 列は citext のため照合は大文字小文字を無視します (atname の UNIQUE
// 制約が強制するのと同じ大小の規則)。未存在は一意性チェックで使う正常なルックアップ結果
// でありエラーではありません。
func (r *UserRepository) FindByAtname(ctx context.Context, atname string) (*model.User, error) {
	row, err := r.q.GetUserByAtname(ctx, atname)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindBySessionToken returns the user that owns the session with the given
// token, or (nil, nil) when no session matches the token (an unknown, stale, or
// forged cookie). It resolves the session and its user in a single JOIN so the
// authentication hot path does not pay two round-trips per request.
//
// [Ja] FindBySessionToken は指定 token のセッションを所有するユーザーを返し、token に
// 一致するセッションが無い場合 (未知 / 失効 / 偽造された Cookie) は (nil, nil) を
// 返します。セッションとそのユーザーを 1 度の JOIN で解決し、認証のホットパスが
// リクエストごとに 2 往復しないようにします。
func (r *UserRepository) FindBySessionToken(ctx context.Context, token string) (*model.User, error) {
	row, err := r.q.GetUserBySessionToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// CreateUserInput holds the identity-level attributes needed to create a user.
// id and the timestamps are assigned by the database.
//
// [Ja] CreateUserInput はユーザー作成に必要な身元レベルの属性を保持します。
// id とタイムスタンプは DB 側で採番されます。
type CreateUserInput struct {
	Email    string
	Atname   string
	Locale   string
	TimeZone string
}

// Create inserts a user and returns it with the database-assigned id and
// timestamps populated.
//
// [Ja] Create はユーザーを挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。
func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (*model.User, error) {
	row, err := r.q.CreateUser(ctx, query.CreateUserParams{
		Email:    input.Email,
		Atname:   input.Atname,
		Locale:   input.Locale,
		TimeZone: input.TimeZone,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// UpdateEmail changes the user's email to the given address and bumps
// updated_at. The email column is citext + UNIQUE, so if another account has
// claimed the same address between validation and this update, the write fails
// with a UNIQUE-violation error the caller must handle (e.g. as a validation
// failure) rather than a silent overwrite.
//
// [Ja] UpdateEmail はユーザーの email を指定アドレスに変更し、updated_at を更新します。
// email 列は citext + UNIQUE のため、検証からこの更新までの間に別アカウントが同じ
// アドレスを取得していた場合、この書き込みは暗黙の上書きではなく UNIQUE 制約違反の
// エラーで失敗します。呼び出し側はこれを (バリデーション失敗などとして) 扱う必要が
// あります。
func (r *UserRepository) UpdateEmail(ctx context.Context, id model.UserID, email string) error {
	return r.q.UpdateUserEmail(ctx, query.UpdateUserEmailParams{
		ID:    uuid.UUID(id),
		Email: email,
	})
}

// SoftDeleteAndAnonymize withdraws the user in one write: it stamps deleted_at
// with the current time and overwrites email and atname with the given anonymized
// values, bumping updated_at. Setting deleted_at makes the account inert
// immediately (authentication lookups exclude soft-deleted rows), while replacing
// email and atname frees those unique values so another account can reclaim them
// before the row is physically purged. The caller supplies collision-free
// anonymized values derived from the user id, so this stays a plain UPDATE with
// no risk of violating the email/atname UNIQUE constraints.
//
// [Ja] SoftDeleteAndAnonymize はユーザーを 1 回の書き込みで退会させます。deleted_at に
// 現在時刻を打ち、email と atname を与えられた匿名値で上書きし、updated_at を更新します。
// deleted_at のセットでアカウントを即座に無効化し (認証ルックアップは論理削除済みの行を
// 除外する)、email と atname の置き換えでそれらの一意な値を解放して、行が物理削除される
// 前に別アカウントが再取得できるようにします。呼び出し側はユーザー id 由来の衝突しない
// 匿名値を渡すため、本処理は email / atname の UNIQUE 制約違反の恐れがない素の UPDATE に
// 留まります。
func (r *UserRepository) SoftDeleteAndAnonymize(ctx context.Context, id model.UserID, email, atname string) error {
	return r.q.SoftDeleteAndAnonymizeUser(ctx, query.SoftDeleteAndAnonymizeUserParams{
		ID:     uuid.UUID(id),
		Email:  email,
		Atname: atname,
	})
}

// PurgeDeletedBefore physically deletes every user soft-deleted before cutoff
// (deleted_at < cutoff), returning how many rows were removed. Each user's child
// rows go with it via ON DELETE CASCADE. This is the second, asynchronous stage of
// withdrawal: the withdrawal request only soft-deletes and anonymizes, and a
// periodic job calls this later to reclaim the storage once the retention window
// has passed. The deleted_at IS NOT NULL predicate lets the query use the partial
// index on deleted_at. The caller passes a time.Time; the pointer the generated
// query expects (deleted_at is a nullable column) is confined to this boundary.
//
// [Ja] PurgeDeletedBefore は cutoff より前に論理削除されたユーザー (deleted_at < cutoff)
// をすべて物理削除し、削除した行数を返します。各ユーザーの子行は ON DELETE CASCADE で
// 一緒に消えます。これは退会の第 2 段階 (非同期) です。退会リクエストは論理削除と匿名化
// だけを行い、保持期間の経過後に定期ジョブが本メソッドを呼んでストレージを回収します。
// deleted_at IS NOT NULL の述語により、クエリは deleted_at の部分インデックスを使えます。
// 呼び出し側は time.Time を渡し、生成クエリが要求するポインタ (deleted_at は nullable な
// カラム) はこの境界に閉じ込めます。
func (r *UserRepository) PurgeDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.q.PurgeUsersDeletedBefore(ctx, &cutoff)
}

// toModel converts a query.User row into a model.User, casting the raw uuid into
// the typed UserID at the repository boundary.
//
// [Ja] toModel は query.User を model.User に変換し、リポジトリの境界で生の uuid を
// 型付きの UserID にキャストします。
func (r *UserRepository) toModel(row query.User) *model.User {
	return &model.User{
		ID:        model.UserID(row.ID),
		Email:     row.Email,
		Atname:    row.Atname,
		Locale:    row.Locale,
		TimeZone:  row.TimeZone,
		DeletedAt: row.DeletedAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
