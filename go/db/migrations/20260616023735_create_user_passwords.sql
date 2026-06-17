-- migrate:up

-- user_passwords holds the native password credential, kept out of the users
-- table so a user's identity and its authentication methods stay separate: an
-- SSO-only user simply has no row here, while a native user has exactly one.
-- email uniqueness lives on users.email (a verified address maps to one
-- account), so this table only needs the digest and a UNIQUE user_id to enforce
-- at most one password per user. password_digest is a bcrypt hash; the plaintext
-- is never stored.
--
-- The user_id FK is ON DELETE CASCADE: a password is pure dependent data with no
-- independent lifecycle and nothing to clean up outside its own row, so it must
-- disappear with the user. CASCADE guarantees this at the database level even if
-- future deletion code forgets to remove the credential first, and the UNIQUE
-- user_id index keeps the cascade from scanning the whole table.
--
-- [Ja] user_passwords は native のパスワード資格情報を保持し、users テーブルから
-- 切り離すことで身元とその認証手段を分離する。SSO のみのユーザーはここに行を持たず、
-- native ユーザーはちょうど 1 行を持つ。email の一意性は users.email が担う (検証済み
-- アドレスが 1 アカウントに対応する) ため、本テーブルはダイジェストと、ユーザーあたり
-- 高々 1 つのパスワードを強制する UNIQUE な user_id だけを持てばよい。password_digest は
-- bcrypt ハッシュで、平文は保存しない。
--
-- user_id の外部キーは ON DELETE CASCADE とする。パスワードは独立したライフサイクルを
-- 持たず、自身の行の外に後始末すべきものもない純粋な従属データのため、ユーザーと一緒に
-- 消えるべきである。CASCADE なら将来の削除コードが先に資格情報を消し忘れても DB レベルで
-- 整合性が保証され、UNIQUE な user_id インデックスによりカスケードがテーブル全体を走査
-- することも避けられる。
CREATE TABLE user_passwords (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    password_digest VARCHAR NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (user_id)
);

-- migrate:down

DROP TABLE IF EXISTS user_passwords;
