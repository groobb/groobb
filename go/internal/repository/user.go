// repositoryパッケージはsqlc生成のクエリをドメインモデルに変換します。
// 各リポジトリは1つのモデルを担当し (model.User <-> UserRepository)、クエリ結果を
// そのモデルに変換することで、DBの詳細を上位層から隠します。
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

// UserRepositoryはsqlc生成のクエリ経由でusersを読み書きします。
type UserRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで
// 書くUserRepositoryを生成します。
func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいUserRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *UserRepository) WithTx(tx *sql.Tx) *UserRepository {
	q := r.writer.WithTx(tx)
	return &UserRepository{reader: q, writer: q}
}

// FindByIDは指定IDのユーザーを返し、存在しない場合は (nil, nil) を返します。
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

// FindByEmailは指定emailのユーザーを返し、存在しない場合は (nil, nil) を
// 返します。email列はNOCASE照合のため、照合は大文字小文字を無視します。
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

// FindByAtnameは指定atnameのユーザーを返し、存在しない場合は (nil, nil) を
// 返します。atname列はNOCASE照合のため大文字小文字を無視します (atnameのUNIQUE
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

// ListByIDsは指定したidのうち、まだ存在するアカウントを返し、スレッドを描く
// 呼び出し元が、これから表示する各投稿を誰が書いたかを1クエリで知れるようにします。
// idを1つずつではなくまとめて取るのは、そうしなければ最大1000件の投稿を載せる
// ページで投稿1件につき1クエリになるためです。
//
// 退会済みのアカウントは、ここの他のルックアップと同じく除外します。退会はアカウントの
// atnameを墓標の値で上書きすることで行われるため、その行を返せば、誰も選んでおらず誰にも
// 辿り着けない名前を呼び出し元へ渡すことになります。書かれたものはいずれにせよ残り、
// アカウントが返ってこないidを呼び出し元が表示する形が、退会した作者の姿そのものです。
// すなわち何にも解決しないidであり、これは物理削除されたアカウントが残すnilのidと
// まったく同じです。
//
// 空のidスライスに対してはクエリを発行せず空のスライスを返します。アカウントを引く
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

// ListPageByAtnamePrefixは、コミュニティのアカウントを1ページ分、登録の新しい
// 順に返し、退会したアカウントを除きます。空のprefixは全員を求め、空でないprefixは
// atnameがそれで始まるアカウントへページを絞ります (大文字小文字は無視します)。
//
// 2つの場合を、すべてに一致するprefixを取る1つの文ではなく別々の文にしています。
// 絞り込みがあるときは範囲がatnameの索引で答えられます。絞り込みが無いときは答えるべき
// 範囲が無く、それでも範囲を求めれば、一覧は50件を返すためにすべてのアカウントをidで
// 並べ替えることになります。主キーを逆にたどれば、ページが埋まった時点で読み終わります。
func (r *UserRepository) ListPageByAtnamePrefix(ctx context.Context, atnamePrefix string, limit, offset int) ([]*model.User, error) {
	var rows []query.User
	var err error
	if atnamePrefix == "" {
		rows, err = r.reader.ListUsersPage(ctx, query.ListUsersPageParams{
			PageSize:   int64(limit),
			PageOffset: int64(offset),
		})
	} else {
		from, to := atnamePrefixBounds(atnamePrefix)
		rows, err = r.reader.ListUsersPageByAtnamePrefix(ctx, query.ListUsersPageByAtnamePrefixParams{
			AtnameFrom: from,
			AtnameTo:   to,
			PageSize:   int64(limit),
			PageOffset: int64(offset),
		})
	}
	if err != nil {
		return nil, err
	}

	users := make([]*model.User, len(rows))
	for i, row := range rows {
		users[i] = r.toModel(row)
	}
	return users, nil
}

// CountByAtnamePrefixは、同じprefixの一覧が何件を対象とするかを返し、ページに
// 番号を振れるようにします。数えるのはListPageByAtnamePrefixが全ページで返すものです。
// 退会したアカウントはここでも除きます。除かなければ、最後のページが、誰にも表示されない
// 行の分まで番号を振られるためです。
func (r *UserRepository) CountByAtnamePrefix(ctx context.Context, atnamePrefix string) (int, error) {
	if atnamePrefix == "" {
		count, err := r.reader.CountUsers(ctx)
		if err != nil {
			return 0, err
		}
		return int(count), nil
	}

	from, to := atnamePrefixBounds(atnamePrefix)
	count, err := r.reader.CountUsersByAtnamePrefix(ctx, query.CountUsersByAtnamePrefixParams{
		AtnameFrom: from,
		AtnameTo:   to,
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// atnamePrefixCeilingは、範囲の上端を閉じるためにprefixの末尾へ足す文字です。
// 存在する最大のコードポイントであるため、atnameが持ちうるどの文字もこれより前に並び、
// atnameがこれ自体を持つことはありません。
const atnamePrefixCeiling = "\U0010FFFF"

// atnamePrefixBoundsはprefixを、それで始まるatnameだけを持つ半開区間
// [from, to) に変換します。あるatnameがprefix以上であり、かつprefixに上端の文字を
// 足したものより前にあるのは、そのatnameがprefixで始まるとき、そのときに限ります。
//
// 前方一致をLIKEではなくこの範囲で書くのは、LIKEが、atnameの文字集合の許す
// アンダースコアを1文字ワイルドカードとして読むためです。それでは "a_b" の検索が "axb"
// も返します。これを文字に戻すにはESCAPE句が要りますが、ESCAPE句は、SQLiteが前方一致の
// LIKEを索引で答えるのをやめる条件の1つであり、一覧はすべてのアカウントを読むことに
// なります。比較はatname列自身のNOCASE照合で行われるため、範囲は大文字小文字を無視し、
// しかもatnameのUNIQUE制約を支える索引で答えられます。
func atnamePrefixBounds(prefix string) (from, to string) {
	return prefix, prefix + atnamePrefixCeiling
}

// FindBySessionTokenは指定tokenのセッションを所有するユーザーを返し、tokenに
// 一致するセッションが無い場合 (未知 / 失効 / 偽造されたCookie) は (nil, nil) を
// 返します。セッションとそのユーザーを1度のJOINで解決し、認証のホットパスが
// リクエストごとに2往復しないようにします。
//
// 停止されたアカウントは、退会したアカウントと同じく何にも解決しません。停止はアカウントが
// 行動することを止めるものである一方、停止された時点でサインインしていた人のCookieはその
// ブラウザに残っているため、セッションが解決できるままであれば、そのCookieがたまたま失効
// するまでそのアカウントは行動できてしまいます。訪問者が代わりに出会うのは、匿名の訪問者
// から見えるコミュニティであり、それは停止されてもなおできることです。
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

// CreateUserInputはユーザー作成に必要な身元レベルの属性を保持します。
// idとタイムスタンプはDB側で採番されます。
type CreateUserInput struct {
	Email    string
	Atname   string
	Locale   model.Locale
	TimeZone string
}

// Createはユーザーを挿入し、DBが採番したidとタイムスタンプを設定した状態で
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

// UpdateEmailはユーザーのemailを指定アドレスに変更し、updated_atを更新します。
// email列はNOCASE照合かつUNIQUEのため、検証からこの更新までの間に別アカウントが同じ
// アドレスを取得していた場合、この書き込みは暗黙の上書きではなくUNIQUE制約違反の
// エラーで失敗します。呼び出し側はこれを (バリデーション失敗などとして) 扱う必要が
// あります。
func (r *UserRepository) UpdateEmail(ctx context.Context, id model.UserID, email string) error {
	return r.writer.UpdateUserEmail(ctx, query.UpdateUserEmailParams{
		ID:    int64(id),
		Email: email,
	})
}

// SoftDeleteAndAnonymizeはユーザーを1回の書き込みで退会させます。deleted_atに
// 現在時刻を打ち、emailとatnameを与えられた匿名値で上書きし、updated_atを更新します。
// deleted_atのセットでアカウントを即座に無効化し (認証ルックアップは論理削除済みの行を
// 除外する)、emailとatnameの置き換えでそれらの一意な値を解放して、行が物理削除される
// 前に別アカウントが再取得できるようにします。
//
// 本処理がUNIQUE制約違反を扱わない素のUPDATEで済むのは、呼び出し側が両方の匿名値を、
// どのアカウントも登録できない形でユーザーidから導くためです。atnameはatnameの形式が
// 拒否する文字を含み、emailは確認コードを配送できない .invalid TLDを使います。登録可能な
// 値を渡す呼び出し側があれば、退会はユーザーには解消できない制約エラーに変わります。
func (r *UserRepository) SoftDeleteAndAnonymize(ctx context.Context, id model.UserID, email, atname string) error {
	return r.writer.SoftDeleteAndAnonymizeUser(ctx, query.SoftDeleteAndAnonymizeUserParams{
		ID:     int64(id),
		Email:  email,
		Atname: atname,
	})
}

// PurgeDeletedBeforeはcutoffより前に論理削除されたユーザー (deleted_at < cutoff)
// をすべて物理削除し、削除した行数を返します。各ユーザーの子行はON DELETE CASCADEで
// 一緒に消えます。これは退会の第2段階 (非同期) です。退会リクエストは論理削除と匿名化
// だけを行い、保持期間の経過後に定期ジョブが本メソッドを呼んでストレージを回収します。
// deleted_at IS NOT NULLの述語により、クエリはdeleted_atの部分インデックスを使えます。
// 呼び出し側はtime.Timeを渡し、保存書式への変換と、生成クエリが要求するポインタ
// (deleted_atはnullableなカラム) への変換はこの境界に閉じ込めます。
func (r *UserRepository) PurgeDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.writer.PurgeUsersDeletedBefore(ctx, sqlitetime.Ptr(&cutoff))
}

// Suspendはアカウントに管理者による停止の時刻を打刻します。列を無条件に書くため、
// 既にある打刻を上書きする呼び出し元は、先に読んでいない呼び出し元です。モデレーションの
// 操作が対象とする状態は、それをコミットする書き込みトランザクションの中で読み、既に停止
// されているアカウントはそこで答えられ、本メソッドには届きません。
//
// 停止はアカウントをサインアウトさせません。サインインしていた人のセッションは、同じ
// トランザクションの中で呼び出し元が削除します。アカウントについてほかに変わるものは
// ありません。emailもatnameもそのままです。停止が止めるのはアカウントが何をできるかで
// あって、それが誰であるかではないためです。
//
// 時刻はmoderation_logs.created_atと同じく、データベースの時計を使います。
func (r *UserRepository) Suspend(ctx context.Context, id model.UserID) error {
	return r.writer.SuspendUser(ctx, int64(id))
}

// Unsuspendは管理者による停止を外し、アカウントが再び行動できるようにします。停止の
// ときに削除したセッションは戻しません。本人はサインインし直すのであり、それがそのアカウント
// が自分のものであることを示すものです。
func (r *UserRepository) Unsuspend(ctx context.Context, id model.UserID) error {
	return r.writer.UnsuspendUser(ctx, int64(id))
}

// toModelはquery.Userをmodel.Userに変換し、リポジトリの境界で生のidを
// 型付きのUserIDに、保存書式の時刻をtime.Timeにキャストします。
func (r *UserRepository) toModel(row query.User) *model.User {
	return &model.User{
		ID:          model.UserID(row.ID),
		Email:       row.Email,
		Atname:      row.Atname,
		Locale:      model.Locale(row.Locale),
		TimeZone:    row.TimeZone,
		DeletedAt:   sqlitetime.TimePtr(row.DeletedAt),
		SuspendedAt: sqlitetime.TimePtr(row.SuspendedAt),

		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
