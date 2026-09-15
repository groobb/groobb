-- +goose Up

-- Groobbインスタンスの初期スキーマ。1つのインスタンスはちょうど1つの
-- コミュニティを運営するため、communitiesはそれを表す1行だけを持ち、インスタンスに
-- 登録したユーザーは全員そのコミュニティのメンバーである。
--
-- 以下のテーブルに共通する規約:
--
--   * 主キーはINTEGER PRIMARY KEYとする。SQLiteはこれをrowidの別名として扱う。
--     AUTOINCREMENTは付けない。INSERTごとにsqlite_sequenceへの書き込みが増える
--     一方、得られるのはidを再利用しない保証だけで、ここではそれに依存するものが
--     無いため。
--   * 時刻の列は宣言型をDATETIMEとし、桁数を固定したISO8601 UTC
--     ("2026-08-21T09:15:00.000Z") を保持する。SQLiteは時刻をテキストとして順序付ける
--     ため、幅の揺れる書式では順序が壊れる。宣言型はsqlcのoverrideがこの列を
--     sqlitetime.Timeへマップする根拠でもある。SQLiteが現在時刻を与える場合は、
--     strftimeで同じ固定書式に揃える。
--   * 真偽値の列はBOOLEANと宣言する。SQLiteはFALSEとTRUEを整数の0と1として
--     保存し、sqlcは宣言型を根拠に列をboolへマップする。
--   * 文字列は長さ指定の無いTEXTとし、長さ制限はアプリケーション側でバリデーション
--     する。
--   * 大文字小文字を区別しない一意性は列のCOLLATE NOCASEで表現し、その上に張る
--     UNIQUEインデックスがこの照合順序で比較する。NOCASEが畳み込むのはASCIIのみ
--     だが、ここでNOCASEを付ける列はいずれもASCIIで足りる。
--   * リストを値に取る列は、JSON配列を保持するTEXT列としjson_validとjson_typeの
--     両方のチェックで守る。SQLiteに配列型が無いため。

-- communitiesはこのインスタンスが運営する唯一のコミュニティを表す。何という
-- コミュニティで、いつ立ち上げられたかを持つ。テーブルを1行に保つのは
-- CHECK (id = 1) である。主キーと組み合わさることで存在しうる行はid = 1だけになる
-- ため、コミュニティを読むコードは「どのコミュニティか」という概念を持たずにこのid
-- で引ける。
CREATE TABLE communities (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- usersは身元のアンカーで、このインスタンス上のアカウント1つにつき1行を持つ。
-- emailはサインインが解決する先のアドレス、atnameは「投稿者が何者か」を示す安定した
-- 対人向けのハンドルである (ADR 0003)。どちらも大文字小文字によらず一意であり、Fooと
-- fooが2つのアカウントになることはない。
--
-- deleted_atは退会したアカウントに印を付ける。退会処理はこれの打刻 (とemail / atname
-- の匿名化) を同期で行ってアカウントを即座に無効化し、行とそのCASCADEする子データの
-- 物理DELETEは定期パージジョブに任せる。インデックスが非NULL行だけを覆うのは、この
-- 列で行を絞り込むのがそのパージだけだからである。アクティブなアカウントを引くクエリは
-- 一意キーで行を特定したうえでdeleted_atを条件に加えるだけで、この索引は使わない。
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

-- user_passwordsはnativeのパスワード資格情報を専用テーブルに持ち、身元と、それを
-- 認証する手段とを分離する。外部プロバイダー経由でのみサインインするユーザーはここに行を
-- 持たない。password_digestはbcryptハッシュで、平文は保存しない。
--
-- 外部キーはON DELETE CASCADEとする。これはusersにぶら下がる以下のすべてのテーブルで
-- 同じで、資格情報は独立したライフサイクルを持たず自身の行の外に後始末すべきものも無い
-- 純粋な従属データのため、削除コードが先に消し忘れてもユーザーと一緒に消えなければ
-- ならない。UNIQUEなuser_idはユーザーあたりのパスワードを1つに抑えると同時に、
-- カスケードがたどるインデックスにもなる。
CREATE TABLE user_passwords (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    password_digest TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id)
);

-- user_sessionsはCookieベースのDBセッションを保持する。サインイン済みの
-- セッション1つにつき1行で、セッションCookieに保持される不透明なtokenをキーと
-- する。tokenはUNIQUEなのでCookieは高々1つのセッションに解決され、ip_address /
-- user_agentはセッションを確立した場所を記録する。signed_in_atは新規の行では
-- created_atと一致するため、同じ既定値を持たせINSERTでは渡さない。
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

-- email_confirmationsは、メールアドレスの管理権を証明する必要があるフローが送る
-- 確認コードを保持する。emailは検証対象のアドレス、eventはフローの名前、codeはユーザーが
-- 入力し返す値である。started_atはコードを発行した時刻で有効期限ウィンドウの基準となり、
-- succeeded_atはコードが受理された時点で打刻する。failed_attempts_countは誤ったコードが
-- 送信された回数を記録し、確認を非アクティブにする上限は列ではなく引き当てるクエリ側に置く。
--
-- user_idがnullableなのは、サインアップがユーザーの存在しない時点で確認を発行するため。
-- サインイン済みのユーザーが申請するフロー (メールアドレスの変更) では行をそのユーザーへ
-- 紐付け、確認ステップが受け渡し用のCookieではなくセッションから行を引けるようにする。
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

-- password_reset_tokensは、ユーザーがパスワードのリセットを申請したときに発行する
-- 使い捨てトークンを保持する。平文のトークンはリセットリンクに入れてメールし、保存は
-- しない。保持するのはSHA-256ハッシュだけなので、データベースが漏えいしても使えるリンクは
-- 渡らない。token_digestがUNIQUEなのはリンクをダイジェストで引くためである。expires_atは
-- リンクが有効な期間を区切り、used_atは使われた時点で打刻してリンクの再利用を防ぐ。
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

-- user_two_factor_authsはユーザーのTOTP設定を保持し、user_passwordsと同じく
-- 資格情報を身元から分離するためusers本体には置かない。行は登録の開始時点で現れ
-- (secretを発行しenabledはfalse)、コードの確認を経てenabledに変わる。recovery_codesは
-- その後1回使い切りのバックアップコードを保持し、使われたコードは取り除かれる。
--
-- secretとrecovery_codesは平文で保存する。Groobbには鍵管理の基盤が無いため、保護は
-- データベースファイルへのアクセス制御に委ねる。recovery_codesの既定値を空のJSON配列と
-- するのは、登録中の行がNULLではなくリストを持つようにするためである。
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

-- rolesはコミュニティが定義するロールを保持し、各ロールは付与する権限スコープを
-- スコープ名のJSON配列として持つ。スコープを割当ではなくロール側に置くのは、ロールで
-- できることを変えるのがメンバーごとではなく1回の更新で済むようにするためである。
-- スコープを行として扱いたいクエリはjson_eachで配列を展開する。
CREATE TABLE roles (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(scopes) AND json_type(scopes) = 'array'),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (name)
);

-- user_rolesはロールをユーザーへ多対多で割り当てる。1人が複数のロールを持て、
-- 1つのロールを複数人が持てる。UNIQUE (user_id, role_id) は同じロールの二重割当を防ぎ、
-- user_idが先頭カラムであることから「このユーザーが持つロール」を引く索引を兼ねる。
-- role_idには単独の索引を張り、ロールの削除でテーブル全体を走査しないようにする。
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
