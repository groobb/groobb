-- +goose Up

-- The initial schema of a Groobb instance. One instance hosts exactly one
-- community, so communities holds the single row that describes it and every
-- user registered on the instance is a member of that community.
--
-- Conventions shared by the tables below:
--
--   * Primary keys are INTEGER PRIMARY KEY, which SQLite makes an alias of the
--     rowid. AUTOINCREMENT is left off: it adds a write to sqlite_sequence per
--     insert and only buys the guarantee that an id is never reused, which
--     nothing here relies on.
--   * Timestamps are declared DATETIME and hold ISO8601 UTC with a fixed number
--     of digits ("2026-08-21T09:15:00.000Z"). SQLite orders them as text, so a
--     format whose width varies would order wrongly. The declared type is also
--     what lets the sqlc override map the column to sqlitetime.Time. When SQLite
--     supplies the current timestamp, strftime writes it in that same fixed
--     format.
--   * Boolean columns are declared BOOLEAN. SQLite stores FALSE and TRUE as the
--     integers 0 and 1, while the declared type makes sqlc map the column to
--     bool.
--   * Text is TEXT without a length modifier; length limits are validated in
--     the application.
--   * Case-insensitive uniqueness is expressed with COLLATE NOCASE on the
--     column, which the UNIQUE index over it then uses for comparison. NOCASE
--     folds ASCII only, which covers every column that carries it here.
--   * A list-valued column is TEXT holding a JSON array, guarded by both
--     json_valid and json_type checks because SQLite has no array type.
--
-- [Ja] Groobb インスタンスの初期スキーマ。1 つのインスタンスはちょうど 1 つの
-- コミュニティを運営するため、communities はそれを表す 1 行だけを持ち、インスタンスに
-- 登録したユーザーは全員そのコミュニティのメンバーである。
--
-- 以下のテーブルに共通する規約:
--
--   * 主キーは INTEGER PRIMARY KEY とする。SQLite はこれを rowid の別名として扱う。
--     AUTOINCREMENT は付けない。INSERT ごとに sqlite_sequence への書き込みが増える
--     一方、得られるのは id を再利用しない保証だけで、ここではそれに依存するものが
--     無いため。
--   * 時刻の列は宣言型を DATETIME とし、桁数を固定した ISO8601 UTC
--     ("2026-08-21T09:15:00.000Z") を保持する。SQLite は時刻をテキストとして順序付ける
--     ため、幅の揺れる書式では順序が壊れる。宣言型は sqlc の override がこの列を
--     sqlitetime.Time へマップする根拠でもある。SQLite が現在時刻を与える場合は、
--     strftime で同じ固定書式に揃える。
--   * 真偽値の列は BOOLEAN と宣言する。SQLite は FALSE と TRUE を整数の 0 と 1 として
--     保存し、sqlc は宣言型を根拠に列を bool へマップする。
--   * 文字列は長さ指定の無い TEXT とし、長さ制限はアプリケーション側でバリデーション
--     する。
--   * 大文字小文字を区別しない一意性は列の COLLATE NOCASE で表現し、その上に張る
--     UNIQUE インデックスがこの照合順序で比較する。NOCASE が畳み込むのは ASCII のみ
--     だが、ここで NOCASE を付ける列はいずれも ASCII で足りる。
--   * リストを値に取る列は、JSON 配列を保持する TEXT 列とし json_valid と json_type の
--     両方のチェックで守る。SQLite に配列型が無いため。

-- communities describes the one community this instance hosts: what it is
-- called and when it was set up. CHECK (id = 1) is what makes the table
-- single-row. Combined with the primary key, the only row that can exist is
-- id = 1, so code that reads the community can select it by that id without a
-- separate "which community" concept.
--
-- [Ja] communities はこのインスタンスが運営する唯一のコミュニティを表す。何という
-- コミュニティで、いつ立ち上げられたかを持つ。テーブルを 1 行に保つのは
-- CHECK (id = 1) である。主キーと組み合わさることで存在しうる行は id = 1 だけになる
-- ため、コミュニティを読むコードは「どのコミュニティか」という概念を持たずにこの id
-- で引ける。
CREATE TABLE communities (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- users is the identity anchor: one row per account on this instance. email is
-- the address a sign-in resolves to, and atname is the stable, human-facing
-- handle that identifies who a poster is (ADR 0003). Both are unique
-- irrespective of letter case, so Foo and foo cannot become two accounts.
--
-- deleted_at marks a withdrawn account. A withdrawal stamps it (and anonymizes
-- email / atname) synchronously so the account is inert at once, while the
-- physical DELETE of the row and its cascading children is left to a periodic
-- purge job. The index covers only the non-null rows: the purge is the only
-- query that selects rows by this column, while the queries that fetch an
-- active account resolve it by a unique key and only test deleted_at
-- afterwards.
--
-- [Ja] users は身元のアンカーで、このインスタンス上のアカウント 1 つにつき 1 行を持つ。
-- email はサインインが解決する先のアドレス、atname は「投稿者が何者か」を示す安定した
-- 対人向けのハンドルである (ADR 0003)。どちらも大文字小文字によらず一意であり、Foo と
-- foo が 2 つのアカウントになることはない。
--
-- deleted_at は退会したアカウントに印を付ける。退会処理はこれの打刻 (と email / atname
-- の匿名化) を同期で行ってアカウントを即座に無効化し、行とその CASCADE する子データの
-- 物理 DELETE は定期パージジョブに任せる。インデックスが非 NULL 行だけを覆うのは、この
-- 列で行を絞り込むのがそのパージだけだからである。アクティブなアカウントを引くクエリは
-- 一意キーで行を特定したうえで deleted_at を条件に加えるだけで、この索引は使わない。
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL COLLATE NOCASE,
    atname TEXT NOT NULL COLLATE NOCASE,
    locale TEXT NOT NULL,
    time_zone TEXT NOT NULL,
    deleted_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (email),
    UNIQUE (atname)
);

CREATE INDEX index_users_on_deleted_at ON users (deleted_at) WHERE deleted_at IS NOT NULL;

-- user_passwords holds the native password credential in its own table so that
-- an identity and the methods used to authenticate it stay separate: a user who
-- only signs in through an external provider simply has no row here.
-- password_digest is a bcrypt hash; the plaintext is never stored.
--
-- The foreign key is ON DELETE CASCADE, as it is on every table below that
-- hangs off users: a credential is pure dependent data with no lifecycle of its
-- own and nothing to clean up outside its own row, so it must disappear with
-- the user even if deletion code forgets to remove it first. The UNIQUE user_id
-- both caps a user at one password and gives the cascade an index to follow.
--
-- [Ja] user_passwords は native のパスワード資格情報を専用テーブルに持ち、身元と、それを
-- 認証する手段とを分離する。外部プロバイダー経由でのみサインインするユーザーはここに行を
-- 持たない。password_digest は bcrypt ハッシュで、平文は保存しない。
--
-- 外部キーは ON DELETE CASCADE とする。これは users にぶら下がる以下のすべてのテーブルで
-- 同じで、資格情報は独立したライフサイクルを持たず自身の行の外に後始末すべきものも無い
-- 純粋な従属データのため、削除コードが先に消し忘れてもユーザーと一緒に消えなければ
-- ならない。UNIQUE な user_id はユーザーあたりのパスワードを 1 つに抑えると同時に、
-- カスケードがたどるインデックスにもなる。
CREATE TABLE user_passwords (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    password_digest TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id)
);

-- user_sessions holds Cookie-backed database sessions: one row per signed-in
-- session, keyed by the opaque token kept in the session cookie. token is
-- UNIQUE so a cookie resolves to at most one session, and ip_address /
-- user_agent record where the session was established. signed_in_at equals
-- created_at on a fresh row, so it carries the same default and is not supplied
-- on insert.
--
-- [Ja] user_sessions は Cookie ベースの DB セッションを保持する。サインイン済みの
-- セッション 1 つにつき 1 行で、セッション Cookie に保持される不透明な token をキーと
-- する。token は UNIQUE なので Cookie は高々 1 つのセッションに解決され、ip_address /
-- user_agent はセッションを確立した場所を記録する。signed_in_at は新規の行では
-- created_at と一致するため、同じ既定値を持たせ INSERT では渡さない。
CREATE TABLE user_sessions (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    signed_in_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (token)
);

CREATE INDEX index_user_sessions_on_user_id ON user_sessions (user_id);

-- email_confirmations holds the verification codes sent by flows that must
-- prove control of an email address. email is the address being verified, event
-- names the flow, and code is what the user types back. started_at is when the
-- code was issued and is the basis of its expiry window; succeeded_at is
-- stamped when the code is accepted. failed_attempts_count records how many
-- wrong codes were submitted, and the limit that turns a confirmation inactive
-- lives in the lookup query rather than in the column.
--
-- user_id is nullable because sign-up issues a confirmation before the user
-- exists. A flow that is requested by a signed-in user (an email change) ties
-- the row to that user instead, which lets the confirming step look the row up
-- from the session rather than from a handoff cookie.
--
-- [Ja] email_confirmations は、メールアドレスの管理権を証明する必要があるフローが送る
-- 確認コードを保持する。email は検証対象のアドレス、event はフローの名前、code はユーザーが
-- 入力し返す値である。started_at はコードを発行した時刻で有効期限ウィンドウの基準となり、
-- succeeded_at はコードが受理された時点で打刻する。failed_attempts_count は誤ったコードが
-- 送信された回数を記録し、確認を非アクティブにする上限は列ではなく引き当てるクエリ側に置く。
--
-- user_id が nullable なのは、サインアップがユーザーの存在しない時点で確認を発行するため。
-- サインイン済みのユーザーが申請するフロー (メールアドレスの変更) では行をそのユーザーへ
-- 紐付け、確認ステップが受け渡し用の Cookie ではなくセッションから行を引けるようにする。
CREATE TABLE email_confirmations (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users (id) ON DELETE CASCADE,
    email TEXT NOT NULL COLLATE NOCASE,
    event TEXT NOT NULL,
    code TEXT NOT NULL,
    failed_attempts_count INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    succeeded_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX index_email_confirmations_on_user_id ON email_confirmations (user_id);

-- password_reset_tokens holds the one-time tokens issued when a user asks to
-- reset their password. The plaintext token is mailed inside the reset link and
-- never stored; only its SHA-256 hash is kept, so a leak of the database does
-- not hand out usable links. token_digest is UNIQUE because the link is looked
-- up by digest. expires_at bounds how long a link works and used_at is stamped
-- when it is spent, so a link cannot be replayed.
--
-- [Ja] password_reset_tokens は、ユーザーがパスワードのリセットを申請したときに発行する
-- 使い捨てトークンを保持する。平文のトークンはリセットリンクに入れてメールし、保存は
-- しない。保持するのは SHA-256 ハッシュだけなので、データベースが漏えいしても使えるリンクは
-- 渡らない。token_digest が UNIQUE なのはリンクをダイジェストで引くためである。expires_at は
-- リンクが有効な期間を区切り、used_at は使われた時点で打刻してリンクの再利用を防ぐ。
CREATE TABLE password_reset_tokens (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_digest TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    used_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (token_digest)
);

CREATE INDEX index_password_reset_tokens_on_user_id ON password_reset_tokens (user_id);

-- user_two_factor_auths holds a user's TOTP setting, kept out of users so the
-- credential material stays separate from identity the way user_passwords does.
-- A row appears when enrollment starts (secret issued, enabled false) and flips
-- to enabled once a code is confirmed; recovery_codes then holds the one-time
-- backup codes and loses a code as it is used.
--
-- secret and recovery_codes are stored as plaintext: Groobb has no key
-- management, so protection rests on access control over the database file.
-- recovery_codes defaults to an empty JSON array so an enrolling row carries a
-- list rather than NULL.
--
-- [Ja] user_two_factor_auths はユーザーの TOTP 設定を保持し、user_passwords と同じく
-- 資格情報を身元から分離するため users 本体には置かない。行は登録の開始時点で現れ
-- (secret を発行し enabled は false)、コードの確認を経て enabled に変わる。recovery_codes は
-- その後 1 回使い切りのバックアップコードを保持し、使われたコードは取り除かれる。
--
-- secret と recovery_codes は平文で保存する。Groobb には鍵管理の基盤が無いため、保護は
-- データベースファイルへのアクセス制御に委ねる。recovery_codes の既定値を空の JSON 配列と
-- するのは、登録中の行が NULL ではなくリストを持つようにするためである。
CREATE TABLE user_two_factor_auths (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    enabled_at DATETIME,
    recovery_codes TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(recovery_codes) AND json_type(recovery_codes) = 'array'),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id)
);

-- roles holds the roles the community defines, each carrying the permission
-- scopes it grants as a JSON array of scope names. Scopes live on the role
-- rather than on the assignment so that changing what a role may do is one
-- update instead of one per member. A query that needs the scopes as rows
-- expands the array with json_each.
--
-- [Ja] roles はコミュニティが定義するロールを保持し、各ロールは付与する権限スコープを
-- スコープ名の JSON 配列として持つ。スコープを割当ではなくロール側に置くのは、ロールで
-- できることを変えるのがメンバーごとではなく 1 回の更新で済むようにするためである。
-- スコープを行として扱いたいクエリは json_each で配列を展開する。
CREATE TABLE roles (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(scopes) AND json_type(scopes) = 'array'),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (name)
);

-- user_roles assigns roles to users, many to many: a user may hold several
-- roles and a role may be held by several users. UNIQUE (user_id, role_id)
-- keeps the same role from being assigned twice and, leading with user_id,
-- doubles as the index for "the roles this user holds". role_id gets an index
-- of its own so that deleting a role does not scan the table.
--
-- [Ja] user_roles はロールをユーザーへ多対多で割り当てる。1 人が複数のロールを持て、
-- 1 つのロールを複数人が持てる。UNIQUE (user_id, role_id) は同じロールの二重割当を防ぎ、
-- user_id が先頭カラムであることから「このユーザーが持つロール」を引く索引を兼ねる。
-- role_id には単独の索引を張り、ロールの削除でテーブル全体を走査しないようにする。
CREATE TABLE user_roles (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id, role_id)
);

CREATE INDEX index_user_roles_on_role_id ON user_roles (role_id);

-- +goose Down

DROP TABLE user_roles;

DROP TABLE roles;

DROP TABLE user_two_factor_auths;

DROP TABLE password_reset_tokens;

DROP TABLE email_confirmations;

DROP TABLE user_sessions;

DROP TABLE user_passwords;

DROP TABLE users;

DROP TABLE communities;
