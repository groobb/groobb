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
	"database/sql"
	"errors"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// UserRepository reads and writes users through sqlc-generated queries.
//
// [Ja] UserRepository は sqlc 生成のクエリ経由で users を読み書きします。
type UserRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserRepository creates a UserRepository that reads through the database's
// read pool and writes through its write pool.
//
// [Ja] NewUserRepository は、データベースの読み取り用プールで読み、書き込み用プールで
// 書く UserRepository を生成します。
func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new UserRepository whose queries run inside tx, so a UseCase
// can enlist this repository in its transaction. The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *UserRepository) WithTx(tx *sql.Tx) *UserRepository {
	q := r.writer.WithTx(tx)
	return &UserRepository{reader: q, writer: q}
}

// FindByID returns the user with the given ID, or (nil, nil) when none exists.
// Absence is a normal lookup outcome, not an error; the caller decides whether
// to treat it as a business-level failure.
//
// [Ja] FindByID は指定 ID のユーザーを返し、存在しない場合は (nil, nil) を返します。
// 未存在は正常なルックアップ結果でありエラーではありません。業務上の失敗として扱うか
// は呼び出し側が判断します。
func (r *UserRepository) FindByID(ctx context.Context, id model.UserID) (*model.User, error) {
	row, err := r.reader.GetUserByID(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindByEmail returns the user with the given email, or (nil, nil) when none
// exists. The email column collates NOCASE, so the match ignores letter case.
//
// [Ja] FindByEmail は指定 email のユーザーを返し、存在しない場合は (nil, nil) を
// 返します。email 列は NOCASE 照合のため、照合は大文字小文字を無視します。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	row, err := r.reader.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindByAtname returns the user with the given atname, or (nil, nil) when none
// exists. The atname column collates NOCASE, so the match ignores letter case (the
// same casing rule the atname UNIQUE constraint enforces). Absence is a normal
// lookup outcome used by the uniqueness check, not an error.
//
// [Ja] FindByAtname は指定 atname のユーザーを返し、存在しない場合は (nil, nil) を
// 返します。atname 列は NOCASE 照合のため大文字小文字を無視します (atname の UNIQUE
// 制約が強制するのと同じ大小の規則)。未存在は一意性チェックで使う正常なルックアップ結果
// でありエラーではありません。
func (r *UserRepository) FindByAtname(ctx context.Context, atname string) (*model.User, error) {
	row, err := r.reader.GetUserByAtname(ctx, atname)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// ListByIDs returns the accounts among the given ids that are still there, so a
// caller rendering a thread learns in one query who wrote each of the posts it
// is about to display. It takes the ids together rather than one at a time
// because the alternative is one query per post on a page that carries up to a
// thousand of them.
//
// A withdrawn account is left out, the same way every other lookup here leaves
// it out. An account withdraws by having its atname overwritten with a
// tombstone, so returning the row would hand the caller a name nobody chose and
// nobody can be reached at. What was written stays either way, and the caller
// showing an id it got no account back for is what a withdrawn author looks
// like: an id that resolves to nothing, exactly like the nil id a purged account
// leaves behind.
//
// An empty slice of ids returns an empty slice without querying: there is
// nothing to look accounts up by.
//
// [Ja] ListByIDs は指定した id のうち、まだ存在するアカウントを返し、スレッドを描く
// 呼び出し元が、これから表示する各投稿を誰が書いたかを 1 クエリで知れるようにします。
// id を 1 つずつではなくまとめて取るのは、そうしなければ最大 1000 件の投稿を載せる
// ページで投稿 1 件につき 1 クエリになるためです。
//
// 退会済みのアカウントは、ここの他のルックアップと同じく除外します。退会はアカウントの
// atname を墓標の値で上書きすることで行われるため、その行を返せば、誰も選んでおらず誰にも
// 辿り着けない名前を呼び出し元へ渡すことになります。書かれたものはいずれにせよ残り、
// アカウントが返ってこない id を呼び出し元が表示する形が、退会した作者の姿そのものです。
// すなわち何にも解決しない id であり、これは物理削除されたアカウントが残す nil の id と
// まったく同じです。
//
// 空の id スライスに対してはクエリを発行せず空のスライスを返します。アカウントを引く
// 手がかりが無いためです。
func (r *UserRepository) ListByIDs(ctx context.Context, ids []model.UserID) ([]*model.User, error) {
	if len(ids) == 0 {
		return []*model.User{}, nil
	}

	rawIDs := make([]int64, len(ids))
	for i, id := range ids {
		rawIDs[i] = int64(id)
	}

	rows, err := r.reader.ListUsersByIDs(ctx, rawIDs)
	if err != nil {
		return nil, err
	}

	users := make([]*model.User, len(rows))
	for i, row := range rows {
		users[i] = r.toModel(row)
	}
	return users, nil
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
	row, err := r.reader.GetUserBySessionToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	Locale   model.Locale
	TimeZone string
}

// Create inserts a user and returns it with the database-assigned id and
// timestamps populated.
//
// [Ja] Create はユーザーを挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。
func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (*model.User, error) {
	row, err := r.writer.CreateUser(ctx, query.CreateUserParams{
		Email:    input.Email,
		Atname:   input.Atname,
		Locale:   string(input.Locale),
		TimeZone: input.TimeZone,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// UpdateEmail changes the user's email to the given address and bumps
// updated_at. The email column collates NOCASE and is UNIQUE, so if another account has
// claimed the same address between validation and this update, the write fails
// with a UNIQUE-violation error the caller must handle (e.g. as a validation
// failure) rather than a silent overwrite.
//
// [Ja] UpdateEmail はユーザーの email を指定アドレスに変更し、updated_at を更新します。
// email 列は NOCASE 照合かつ UNIQUE のため、検証からこの更新までの間に別アカウントが同じ
// アドレスを取得していた場合、この書き込みは暗黙の上書きではなく UNIQUE 制約違反の
// エラーで失敗します。呼び出し側はこれを (バリデーション失敗などとして) 扱う必要が
// あります。
func (r *UserRepository) UpdateEmail(ctx context.Context, id model.UserID, email string) error {
	return r.writer.UpdateUserEmail(ctx, query.UpdateUserEmailParams{
		ID:    int64(id),
		Email: email,
	})
}

// SoftDeleteAndAnonymize withdraws the user in one write: it stamps deleted_at
// with the current time and overwrites email and atname with the given anonymized
// values, bumping updated_at. Setting deleted_at makes the account inert
// immediately (authentication lookups exclude soft-deleted rows), while replacing
// email and atname frees those unique values so another account can reclaim them
// before the row is physically purged.
//
// This stays a plain UPDATE with no UNIQUE-violation handling because the caller
// derives both anonymized values from the user id in a shape no account can
// register: the atname carries a character the atname format rejects, and the
// email uses the .invalid TLD, which no confirmation code can be delivered to.
// A caller that supplies a registrable value instead would turn a withdrawal
// into a constraint error the user cannot resolve.
//
// [Ja] SoftDeleteAndAnonymize はユーザーを 1 回の書き込みで退会させます。deleted_at に
// 現在時刻を打ち、email と atname を与えられた匿名値で上書きし、updated_at を更新します。
// deleted_at のセットでアカウントを即座に無効化し (認証ルックアップは論理削除済みの行を
// 除外する)、email と atname の置き換えでそれらの一意な値を解放して、行が物理削除される
// 前に別アカウントが再取得できるようにします。
//
// 本処理が UNIQUE 制約違反を扱わない素の UPDATE で済むのは、呼び出し側が両方の匿名値を、
// どのアカウントも登録できない形でユーザー id から導くためです。atname は atname の形式が
// 拒否する文字を含み、email は確認コードを配送できない .invalid TLD を使います。登録可能な
// 値を渡す呼び出し側があれば、退会はユーザーには解消できない制約エラーに変わります。
func (r *UserRepository) SoftDeleteAndAnonymize(ctx context.Context, id model.UserID, email, atname string) error {
	return r.writer.SoftDeleteAndAnonymizeUser(ctx, query.SoftDeleteAndAnonymizeUserParams{
		ID:     int64(id),
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
// index on deleted_at. The caller passes a time.Time; converting it to the stored
// timestamp format, and to the pointer the generated query expects (deleted_at is a
// nullable column), is confined to this boundary.
//
// [Ja] PurgeDeletedBefore は cutoff より前に論理削除されたユーザー (deleted_at < cutoff)
// をすべて物理削除し、削除した行数を返します。各ユーザーの子行は ON DELETE CASCADE で
// 一緒に消えます。これは退会の第 2 段階 (非同期) です。退会リクエストは論理削除と匿名化
// だけを行い、保持期間の経過後に定期ジョブが本メソッドを呼んでストレージを回収します。
// deleted_at IS NOT NULL の述語により、クエリは deleted_at の部分インデックスを使えます。
// 呼び出し側は time.Time を渡し、保存書式への変換と、生成クエリが要求するポインタ
// (deleted_at は nullable なカラム) への変換はこの境界に閉じ込めます。
func (r *UserRepository) PurgeDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.writer.PurgeUsersDeletedBefore(ctx, sqlitetime.Ptr(&cutoff))
}

// toModel converts a query.User row into a model.User, casting the raw id into the
// typed UserID and the stored timestamps back into time.Time at the repository
// boundary.
//
// [Ja] toModel は query.User を model.User に変換し、リポジトリの境界で生の id を
// 型付きの UserID に、保存書式の時刻を time.Time にキャストします。
func (r *UserRepository) toModel(row query.User) *model.User {
	return &model.User{
		ID:        model.UserID(row.ID),
		Email:     row.Email,
		Atname:    row.Atname,
		Locale:    model.Locale(row.Locale),
		TimeZone:  row.TimeZone,
		DeletedAt: sqlitetime.TimePtr(row.DeletedAt),
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
