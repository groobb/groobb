// databaseパッケージは、アプリケーションが動作するSQLiteデータベースを開き、
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

	// pure Goドライバはdatabase/sqlへ "sqlite" という名前で自身を登録する
	// 副作用のためにimportする。CGOを要するドライバではなくこれを選ぶのは、
	// サーバーを静的リンクされた単一バイナリとして配布できるようにするため。
	_ "modernc.org/sqlite"
)

const (
	// driverNameはmodernc.org/sqliteがdatabase/sqlに登録する名前です。
	driverName = "sqlite"

	// busyTimeoutは、他のコネクションが保持するロックの解放を待つ時間です。
	// これを過ぎるとSQLITE_BUSYで諦めます。SQLiteはデータベースファイル全体で
	// ライターを直列化するため、リクエストが並行するとロック競合は日常的に起きます。
	// リクエストを失敗させるより待つほうが望ましいため、タイムアウトを設けます。
	busyTimeout = 5 * time.Second

	// minReaderConnsは読み取りプールのサイズの下限です。読み取りは多くの場合
	// CPUで完結するためプールサイズはコア数を基準にしますが、1コアのホストでも、
	// ある読み取りがディスクを待つ間に他のリクエストを処理し続けられる程度の本数は
	// 必要です。
	minReaderConns = 4
)

// DBは1つのSQLiteデータベースファイルを支える2つの接続プールを保持します。
//
// SQLiteはファイル全体で同時に1つのライターしか許さないため、2つのプールは
// 交換可能ではありません。書き込み用プールはコネクションを1本に制限して
// トランザクションをBEGIN IMMEDIATEで開始し、読み取り用プールはWALモードが
// ライターと並行して走らせられる読み取りを複数のコネクションで処理します。また、
// 読み取り用プールはすべてのコネクションでquery_onlyを有効にするため、誤って
// 振り分けられた文はsingle-writer設計を迂回せず失敗します。
type DB struct {
	// Writerはデータベースを変更するすべての文を実行します。
	Writer *sql.DB

	// Readerは読み取り専用の文を実行します。
	Reader *sql.DB
}

// OpenはpathのSQLiteデータベースファイルを開き、書き込み用と読み取り用の
// プールを返します。返す前に両方へpingします。起動時にpingすることで、設定ミスや
// 開けないデータベースを最初のリクエストでのエラーとしてではなく早期に検知できます。
// またdatabase/sqlはコネクションを遅延して開くため、pingは接続時PRAGMAを適用する
// 契機でもあります。返したDBは呼び出し側の所有物であり、クローズの責務も呼び出し側に
// あります。
func Open(ctx context.Context, path string) (*DB, error) {
	// pathが空だとSQLiteはクローズ時に削除されるprivateな一時データベースを
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

// Closeは両方のプールをクローズし、いずれかのエラーを返します。
func (db *DB) Close() error {
	return errors.Join(db.Reader.Close(), db.Writer.Close())
}

// openPoolはdsnに対して最大maxConns本のコネクションを持つプールを開き、
// pingで疎通を確認します。
//
// アイドル数の上限を最大接続数と同じに引き上げるのは、リクエストの合間にコネクションを
// 閉じて開き直すのではなく再利用するためです。ここでの開き直しはただではなく、新しい
// コネクションのたびにDSNが持つPRAGMAを実行し直すことになります。
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

// writerDataSourceNameは書き込み用プールのDSNを組み立てます。
//
// BEGINでただちに書き込みロックを取り、トランザクション内の最初の書き込みで昇格する
// 形にはしません。SQLiteは昇格の待機にbusy_timeoutを適用しないため、deferredな
// 書き込みトランザクションはロックを保持しているライターを待たずにSQLITE_BUSYで
// 諦めてしまいます。
func writerDataSourceName(path string) string {
	return dataSourceName(path, url.Values{"_txlock": {"immediate"}})
}

// readerDataSourceNameは読み取り用プールのDSNを組み立てます。
//
// query_onlyにより、誤って読み取り用プールへ振り分けられた文は、複数あるコネクションの
// どれかから書き込んでsingle-writer設計を迂回するのではなく、コネクション上で失敗
// します。
func readerDataSourceName(path string) string {
	return dataSourceName(path, url.Values{"_query_only": {"on"}})
}

// dataSourceNameはpathのデータベースファイルに対するドライバのDSNを
// 組み立てます。extraは両方のプールで共通のパラメーターに上書きマージされるもので、
// 書き込み用と読み取り用のどちらのコネクションなのかを各プールが表明する場所です。
//
// PRAGMAを接続後に実行せずDSNに載せるのは、database/sqlがプールからコネクションを
// 配り、必要に応じて新しいコネクションを開くためです。Openの後に1度だけPRAGMAを
// 実行しても、それが効くのは1本のコネクションだけで、プールの残りはSQLiteの既定値の
// ままになります。ここで使うキーはドライバのショートハンドで、ドライバはどれかを実行する
// 前に値を検証するため、受け付けられない値を渡すとコネクション自体が失敗します。検証されない
// `_pragma` キーは文字列のまま実行されるため、途中まで適用された状態のコネクションが
// 残りえます。
//
// pathは "file:" URIのパス要素としてクエリと分離してエンコードします。ドライバは
// 素のファイル名でも最初の疑問符から後ろをクエリとして扱うため、URIの区切り文字を
// 含む名前を保つにはパーセントエンコードが必要です。絶対パスは空のオーソリティに続けて
// 書き ("file:///var/lib/groobb.sqlite")、スラッシュ2つで始まる名前がオーソリティと
// 読まれないようにします。相対パスはオーソリティが最初のセグメントを飲み込んでしまうため
// この形にできず、代わりに "./" を前置します。これにより ":memory:" や "file::memory:"
// のような名前もSQLiteの特別なインメモリモードではなく通常のファイルシステム上のパスに
// なります。
func dataSourceName(path string, extra url.Values) string {
	params := url.Values{}
	params.Set("_busy_timeout", strconv.FormatInt(busyTimeout.Milliseconds(), 10))

	// WALは書き込み中でも読み取りを走らせられるようにするもので、読み取りプールを
	// 分ける価値はここから来る。NORMALはWALと組み合わせる前提の耐久性レベルで、OSの
	// クラッシュや電源断で直近のトランザクションを失う可能性があるだけで、データベースが
	// 壊れることはない。
	params.Set("_journal_mode", "WAL")
	params.Set("_synchronous", "NORMAL")

	// SQLiteは外部キー制約をコネクションごとに有効化しない限り無視する。
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
