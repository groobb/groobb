-- +goose Up

-- Create the tables River (the background job queue) needs to operate. River
-- ships its own migrator, but Groobb applies every schema change through goose
-- so that one mechanism owns the database file; the River DDL is therefore
-- vendored here as a single migration. The statements are the schema River
-- v0.40.0's SQLite driver reaches at migration version 7 (the version the
-- linked client library expects), and the closing INSERT records all seven line
-- migrations as applied so the library does not migrate again at runtime. River
-- owns these tables at runtime through riversqlite; the application never
-- queries them itself.
--
-- What has to match River is the set of columns and their types, not the order
-- they are declared in: River's queries are generated from `SELECT *`, and sqlc
-- expands that into a fixed list of column names at build time, so SQLite
-- returns them in the order the query names rather than the order the table
-- declares. A column River names that this table lacks, or spells differently,
-- fails its queries instead. TestRiverSchemaRoundTripsAJob in internal/worker
-- covers that by carrying a job from insertion to a worker, which is where such
-- a mismatch surfaces: applying this migration on its own does not.
--
-- The statements are nevertheless taken from the migrator verbatim rather than
-- tidied up, because reading its output is what keeps details like river_job's
-- max_attempts sitting last (version 7 re-adds it as a new column to give it a
-- default, and SQLite appends an added column) from being lost to a rewrite that
-- looks more natural.
--
-- When River is bumped and a new migration version N appears, generate the SQL
-- the same way rather than by hand: run River's migrator against a scratch
-- SQLite database with the new library version, read back the DDL SQLite stores
-- in sqlite_master, add it as a new goose migration that appends
-- `INSERT INTO river_migration (line, version) VALUES ('main', N);` (DELETE on
-- down), and bump appliedRiverMigrationVersion in internal/worker to N.
--
-- [Ja] バックグラウンドジョブキュー River が動作するために必要なテーブルを作成する。
-- River は自前のマイグレータを持つが、Groobb はすべてのスキーマ変更を goose 経由で
-- 適用し、データベースファイルを 1 つの仕組みが所有する形にしているため、River の DDL は
-- 1 本のマイグレーションとしてここに取り込む。各文は River v0.40.0 の SQLite ドライバが
-- マイグレーションバージョン 7 (リンク済みのクライアントライブラリが期待するバージョン) で
-- 到達するスキーマであり、末尾の INSERT で 7 つのラインマイグレーションがすべて適用済みで
-- あることを記録して、ライブラリが実行時に再度マイグレーションを試みないようにする。
-- これらのテーブルは実行時に riversqlite 経由で River が所有し、アプリケーション自身が
-- クエリすることはない。
--
-- River に合っていなければならないのはカラムの集合と型であって、宣言の並び順ではない。
-- River 自身のクエリは `SELECT *` から生成されるが、sqlc がビルド時にこれをカラム名の
-- 固定リストへ展開するため、SQLite はテーブルの宣言順ではなくクエリが名指しした順で値を
-- 返す。代わりに、River が名指しするカラムがこのテーブルに無い、または綴りが違う場合に
-- クエリが失敗する。internal/worker の TestRiverSchemaRoundTripsAJob がジョブを投入から
-- ワーカーまで運ぶことでこれを検査する。そこがこの食い違いの現れる場所であり、本
-- マイグレーションを適用するだけでは現れない。
--
-- それでも各文を整形せずマイグレータの出力のまま取り込むのは、その出力を読むことが、
-- river_job の max_attempts が末尾にある (バージョン 7 が既定値を与えるためにこの列を
-- 新しいカラムとして追加し直し、SQLite は追加されたカラムを末尾に置く) といった細部を、
-- 自然に見える書き直しで失わずに済む方法だからである。
--
-- River を bump して新しいマイグレーションバージョン N が増えたときは、手書きではなく
-- 同じ手順で SQL を生成する。新しいライブラリバージョンで River のマイグレータを使い捨ての
-- SQLite データベースに対して実行し、SQLite が sqlite_master に保持している DDL を読み出し、
-- 末尾に `INSERT INTO river_migration (line, version) VALUES ('main', N);` を追記する
-- 新しい goose マイグレーションとして追加し (down では DELETE)、internal/worker の
-- appliedRiverMigrationVersion を N に更新する。

CREATE TABLE river_migration (
    line text NOT NULL,
    version integer NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT line_length CHECK (length(line) > 0 AND length(line) < 128),
    CONSTRAINT version_gte_1 CHECK (version >= 1),
    PRIMARY KEY (line, version)
);

CREATE TABLE river_leader (
    elected_at timestamp NOT NULL,
    expires_at timestamp NOT NULL,
    leader_id text NOT NULL,
    name text PRIMARY KEY NOT NULL DEFAULT 'default' CHECK (name = 'default'),
    CONSTRAINT name_length CHECK (length(name) > 0 AND length(name) < 128),
    CONSTRAINT leader_id_length CHECK (length(leader_id) > 0 AND length(leader_id) < 128)
);

CREATE TABLE river_queue (
    name text PRIMARY KEY NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata blob NOT NULL DEFAULT (jsonb('{}')),
    paused_at timestamp,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE river_job (
    id integer PRIMARY KEY,
    args blob NOT NULL DEFAULT (jsonb('{}')),
    attempt integer NOT NULL DEFAULT 0,
    attempted_at timestamp,
    attempted_by blob,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    errors blob,
    finalized_at timestamp,
    kind text NOT NULL,
    metadata blob NOT NULL DEFAULT (jsonb('{}')),
    priority integer NOT NULL DEFAULT 1,
    queue text NOT NULL DEFAULT 'default',
    state text NOT NULL DEFAULT 'available',
    scheduled_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tags blob NOT NULL DEFAULT (jsonb('[]')),
    unique_key blob,
    unique_states integer,
    max_attempts integer NOT NULL DEFAULT 25,
    CONSTRAINT finalized_or_finalized_at_null CHECK (
        (finalized_at IS NULL AND state NOT IN ('cancelled', 'completed', 'discarded')) OR
        (finalized_at IS NOT NULL AND state IN ('cancelled', 'completed', 'discarded'))
    ),
    CONSTRAINT priority_in_range CHECK (priority >= 1 AND priority <= 4),
    CONSTRAINT queue_length CHECK (length(queue) > 0 AND length(queue) < 128),
    CONSTRAINT kind_length CHECK (length(kind) > 0 AND length(kind) < 128),
    CONSTRAINT state_valid CHECK (state IN ('available', 'cancelled', 'completed', 'discarded', 'pending', 'retryable', 'running', 'scheduled'))
);

CREATE INDEX river_job_kind ON river_job (kind);

CREATE INDEX river_job_state_and_finalized_at_index ON river_job (state, finalized_at) WHERE finalized_at IS NOT NULL;

CREATE INDEX river_job_prioritized_fetching_index ON river_job (state, queue, priority, scheduled_at, id);

CREATE UNIQUE INDEX river_job_unique_idx ON river_job (unique_key)
    WHERE unique_key IS NOT NULL
        AND unique_states IS NOT NULL
        AND CASE state
            WHEN 'available' THEN unique_states & (1 << 0)
            WHEN 'cancelled' THEN unique_states & (1 << 1)
            WHEN 'completed' THEN unique_states & (1 << 2)
            WHEN 'discarded' THEN unique_states & (1 << 3)
            WHEN 'pending'   THEN unique_states & (1 << 4)
            WHEN 'retryable' THEN unique_states & (1 << 5)
            WHEN 'running'   THEN unique_states & (1 << 6)
            WHEN 'scheduled' THEN unique_states & (1 << 7)
            ELSE 0
        END >= 1;

CREATE TABLE river_notification (
    id integer PRIMARY KEY AUTOINCREMENT,
    created_at timestamp NOT NULL DEFAULT (datetime('now', 'subsec')),
    payload text NOT NULL,
    topic text NOT NULL,
    CONSTRAINT topic_length CHECK (length(topic) > 0 AND length(topic) < 128)
);

CREATE INDEX river_notification_created_at_idx ON river_notification (created_at);

CREATE INDEX river_notification_topic_id_idx ON river_notification (topic, id);

INSERT INTO river_migration (line, version)
VALUES ('main', 1), ('main', 2), ('main', 3), ('main', 4), ('main', 5), ('main', 6), ('main', 7);

-- +goose Down

DROP TABLE river_notification;

DROP TABLE river_job;

DROP TABLE river_queue;

DROP TABLE river_leader;

DROP TABLE river_migration;
