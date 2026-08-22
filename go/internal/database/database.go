// Package database opens the SQLite database the application runs on and
// applies the migrations embedded in the binary.
//
// [Ja] database パッケージは、アプリケーションが動作する SQLite データベースを開き、
// バイナリに埋め込まれたマイグレーションを適用します。
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	// The pure Go driver is imported for its side effect of registering itself
	// with database/sql under the name "sqlite". It is preferred over the
	// CGO-backed drivers so that the server can be shipped as a single
	// statically linked binary.
	//
	// [Ja] pure Go ドライバは database/sql へ "sqlite" という名前で自身を登録する
	// 副作用のために import する。CGO を要するドライバではなくこれを選ぶのは、
	// サーバーを静的リンクされた単一バイナリとして配布できるようにするため。
	_ "modernc.org/sqlite"
)

const (
	// driverName is the name modernc.org/sqlite registers with database/sql.
	//
	// [Ja] driverName は modernc.org/sqlite が database/sql に登録する名前です。
	driverName = "sqlite"

	// busyTimeout is how long a connection waits for a lock held by another
	// connection before giving up with SQLITE_BUSY. SQLite serializes writers
	// across the whole database file, so brief contention is expected under
	// concurrent requests and waiting it out is preferable to failing the
	// request.
	//
	// [Ja] busyTimeout は、他のコネクションが保持するロックの解放を待つ時間です。
	// これを過ぎると SQLITE_BUSY で諦めます。SQLite はデータベースファイル全体で
	// ライターを直列化するため、リクエストが並行するとロック競合は日常的に起きます。
	// リクエストを失敗させるより待つほうが望ましいため、タイムアウトを設けます。
	busyTimeout = 5 * time.Second

	// minReaderConns is the floor for the read pool size. Reads are served from
	// the CPU in the common case, so the pool is sized by the number of cores,
	// but a single-core host still needs a few connections to keep serving
	// requests while one read waits on disk.
	//
	// [Ja] minReaderConns は読み取りプールのサイズの下限です。読み取りは多くの場合
	// CPU で完結するためプールサイズはコア数を基準にしますが、1 コアのホストでも、
	// ある読み取りがディスクを待つ間に他のリクエストを処理し続けられる程度の本数は
	// 必要です。
	minReaderConns = 4
)

// DB holds the two connection pools that back a single SQLite database file.
//
// SQLite allows only one writer at a time for the whole file, so the two pools
// are not interchangeable: the write pool is capped at a single connection and
// begins its transactions with BEGIN IMMEDIATE, while the read pool serves
// concurrent readers that WAL mode lets run alongside the writer. The read pool
// also enables query_only on every connection so that a statement accidentally
// routed to it fails instead of bypassing the single-writer design.
//
// [Ja] DB は 1 つの SQLite データベースファイルを支える 2 つの接続プールを保持します。
//
// SQLite はファイル全体で同時に 1 つのライターしか許さないため、2 つのプールは
// 交換可能ではありません。書き込み用プールはコネクションを 1 本に制限して
// トランザクションを BEGIN IMMEDIATE で開始し、読み取り用プールは WAL モードが
// ライターと並行して走らせられる読み取りを複数のコネクションで処理します。また、
// 読み取り用プールはすべてのコネクションで query_only を有効にするため、誤って
// 振り分けられた文は single-writer 設計を迂回せず失敗します。
type DB struct {
	// Writer runs every statement that modifies the database.
	//
	// [Ja] Writer はデータベースを変更するすべての文を実行します。
	Writer *sql.DB

	// Reader runs read-only statements.
	//
	// [Ja] Reader は読み取り専用の文を実行します。
	Reader *sql.DB
}

// Open opens the SQLite database file at path and returns its write and read
// pools, pinging both before returning. Pinging on startup makes a
// misconfigured or unopenable database fail fast instead of surfacing as errors
// on the first request; it is also what applies the connection PRAGMAs, since
// database/sql opens connections lazily. The caller owns the returned DB and
// must close it.
//
// [Ja] Open は path の SQLite データベースファイルを開き、書き込み用と読み取り用の
// プールを返します。返す前に両方へ ping します。起動時に ping することで、設定ミスや
// 開けないデータベースを最初のリクエストでのエラーとしてではなく早期に検知できます。
// また database/sql はコネクションを遅延して開くため、ping は接続時 PRAGMA を適用する
// 契機でもあります。返した DB は呼び出し側の所有物であり、クローズの責務も呼び出し側に
// あります。
func Open(ctx context.Context, path string) (*DB, error) {
	// An empty path makes SQLite open a private temporary database that is
	// deleted on close, so every write would silently vanish. Reject it here
	// rather than letting an unset setting look like a working database.
	//
	// [Ja] path が空だと SQLite はクローズ時に削除される private な一時データベースを
	// 開くため、書き込みが黙って消える。設定漏れが動作するデータベースに見えてしまう
	// ことを防ぐため、ここで弾く。
	if path == "" {
		return nil, errors.New("the database file path is empty")
	}

	writer, err := openPool(ctx, writerDataSourceName(path), 1)
	if err != nil {
		return nil, fmt.Errorf("failed to open the write pool: %w", err)
	}

	readerConns := max(runtime.NumCPU(), minReaderConns)
	reader, err := openPool(ctx, readerDataSourceName(path), readerConns)
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to open the read pool: %w", err)
	}

	return &DB{Writer: writer, Reader: reader}, nil
}

// Close closes both pools and reports any error from either.
//
// [Ja] Close は両方のプールをクローズし、いずれかのエラーを返します。
func (db *DB) Close() error {
	return errors.Join(db.Reader.Close(), db.Writer.Close())
}

// openPool opens a pool of at most maxConns connections for dsn and verifies
// connectivity with a ping.
//
// The idle limit is raised to match the open limit so that a connection is
// reused instead of being closed and reopened between requests. Reopening is
// not free here: every new connection re-runs the PRAGMAs carried by the DSN.
//
// [Ja] openPool は dsn に対して最大 maxConns 本のコネクションを持つプールを開き、
// ping で疎通を確認します。
//
// アイドル数の上限を最大接続数と同じに引き上げるのは、リクエストの合間にコネクションを
// 閉じて開き直すのではなく再利用するためです。ここでの開き直しはただではなく、新しい
// コネクションのたびに DSN が持つ PRAGMA を実行し直すことになります。
func openPool(ctx context.Context, dsn string, maxConns int) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open the database: %w", err)
	}

	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping the database: %w", err)
	}

	return db, nil
}

// writerDataSourceName builds the DSN for the write pool.
//
// BEGIN takes the write lock immediately rather than upgrading to it on the
// first write of the transaction. SQLite does not apply busy_timeout to that
// upgrade, so a deferred write transaction gives up with SQLITE_BUSY instead of
// waiting for the writer that holds the lock.
//
// [Ja] writerDataSourceName は書き込み用プールの DSN を組み立てます。
//
// BEGIN でただちに書き込みロックを取り、トランザクション内の最初の書き込みで昇格する
// 形にはしません。SQLite は昇格の待機に busy_timeout を適用しないため、deferred な
// 書き込みトランザクションはロックを保持しているライターを待たずに SQLITE_BUSY で
// 諦めてしまいます。
func writerDataSourceName(path string) string {
	return dataSourceName(path, url.Values{"_txlock": {"immediate"}})
}

// readerDataSourceName builds the DSN for the read pool.
//
// query_only makes a statement accidentally routed to the read pool fail on the
// connection instead of writing from one of its several connections and
// bypassing the single-writer design.
//
// [Ja] readerDataSourceName は読み取り用プールの DSN を組み立てます。
//
// query_only により、誤って読み取り用プールへ振り分けられた文は、複数あるコネクションの
// どれかから書き込んで single-writer 設計を迂回するのではなく、コネクション上で失敗
// します。
func readerDataSourceName(path string) string {
	return dataSourceName(path, url.Values{"_query_only": {"on"}})
}

// dataSourceName builds the driver DSN for the database file at path. The extra
// parameters are merged over the ones both pools share, and are where each pool
// states what makes it a write or a read connection.
//
// The PRAGMAs travel in the DSN rather than being executed after connecting,
// because database/sql hands out connections from a pool and opens new ones on
// demand: a PRAGMA run once after Open would apply to one connection and leave
// the rest of the pool with SQLite's defaults. The keys used here are the
// driver's shorthands, whose values it validates before executing any of them,
// so a value it does not accept fails the connection outright. The unvalidated
// "_pragma" key is instead executed verbatim, which can leave a connection
// configured only partway.
//
// The path is encoded separately as the path component of a "file:" URI. The
// driver treats the first question mark in every DSN as the start of the query,
// even for a plain filename, so percent-encoding is required to preserve names
// containing URI delimiters. An absolute path is written after an empty
// authority ("file:///var/lib/groobb.sqlite") so that a name beginning with two
// slashes is not read as one. A relative path cannot be written that way, since
// the authority would swallow its first segment, so it is prefixed with "./"
// instead; that also keeps names such as ":memory:" and "file::memory:"
// ordinary filesystem paths rather than SQLite's special in-memory modes.
//
// [Ja] dataSourceName は path のデータベースファイルに対するドライバの DSN を
// 組み立てます。extra は両方のプールで共通のパラメーターに上書きマージされるもので、
// 書き込み用と読み取り用のどちらのコネクションなのかを各プールが表明する場所です。
//
// PRAGMA を接続後に実行せず DSN に載せるのは、database/sql がプールからコネクションを
// 配り、必要に応じて新しいコネクションを開くためです。Open の後に 1 度だけ PRAGMA を
// 実行しても、それが効くのは 1 本のコネクションだけで、プールの残りは SQLite の既定値の
// ままになります。ここで使うキーはドライバのショートハンドで、ドライバはどれかを実行する
// 前に値を検証するため、受け付けられない値を渡すとコネクション自体が失敗します。検証されない
// `_pragma` キーは文字列のまま実行されるため、途中まで適用された状態のコネクションが
// 残りえます。
//
// path は "file:" URI のパス要素としてクエリと分離してエンコードします。ドライバは
// 素のファイル名でも最初の疑問符から後ろをクエリとして扱うため、URI の区切り文字を
// 含む名前を保つにはパーセントエンコードが必要です。絶対パスは空のオーソリティに続けて
// 書き ("file:///var/lib/groobb.sqlite")、スラッシュ 2 つで始まる名前がオーソリティと
// 読まれないようにします。相対パスはオーソリティが最初のセグメントを飲み込んでしまうため
// この形にできず、代わりに "./" を前置します。これにより ":memory:" や "file::memory:"
// のような名前も SQLite の特別なインメモリモードではなく通常のファイルシステム上のパスに
// なります。
func dataSourceName(path string, extra url.Values) string {
	params := url.Values{}
	params.Set("_busy_timeout", strconv.FormatInt(busyTimeout.Milliseconds(), 10))

	// WAL lets readers run while a write is in flight, which is what makes the
	// separate read pool worthwhile. NORMAL is the durability level WAL is meant
	// to be paired with: it only risks losing the most recent transactions on an
	// OS crash or power loss, and never corrupts the database.
	//
	// [Ja] WAL は書き込み中でも読み取りを走らせられるようにするもので、読み取りプールを
	// 分ける価値はここから来る。NORMAL は WAL と組み合わせる前提の耐久性レベルで、OS の
	// クラッシュや電源断で直近のトランザクションを失う可能性があるだけで、データベースが
	// 壊れることはない。
	params.Set("_journal_mode", "WAL")
	params.Set("_synchronous", "NORMAL")

	// SQLite ignores foreign key constraints unless they are switched on per
	// connection.
	//
	// [Ja] SQLite は外部キー制約をコネクションごとに有効化しない限り無視する。
	params.Set("_foreign_keys", "on")

	maps.Copy(params, extra)

	isAbs := filepath.IsAbs(path)
	uriPath := filepath.ToSlash(path)
	if !isAbs {
		uriPath = "./" + uriPath
	}

	dsn := url.URL{
		OmitHost: !isAbs,
		Scheme:   "file",
		Path:     uriPath,
		RawQuery: params.Encode(),
	}

	return dsn.String()
}
