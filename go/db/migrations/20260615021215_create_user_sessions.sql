-- migrate:up

-- user_sessions holds Cookie-backed database sessions: each row is one signed-in
-- session for a user, keyed by an opaque token kept in the session cookie. token
-- is UNIQUE so a cookie resolves to at most one session, and user_id is indexed
-- so a user's sessions can be looked up and revoked. ip_address / user_agent
-- record where the session was established. signed_in_at is the sign-in moment;
-- for a freshly created row it equals created_at, so it defaults to NOW() like
-- the other timestamps and is not supplied on insert.
--
-- The user_id FK is ON DELETE CASCADE: a session is pure dependent data with no
-- independent lifecycle and nothing to clean up outside its own row, so a user's
-- sessions must disappear with the user. CASCADE guarantees this at the database
-- level even if future deletion code forgets to remove the sessions first, and
-- the user_id index keeps the cascade from scanning the whole table.
--
-- [Ja] user_sessions は Cookie ベースの DB セッションを保持する。各行はユーザーの
-- 1 つのサインイン済みセッションで、セッション Cookie に保持される不透明な token を
-- キーとする。token は UNIQUE なので Cookie は高々 1 つのセッションに解決され、
-- user_id にインデックスを張ることでユーザーのセッションを引いて失効できる。
-- ip_address / user_agent はセッションを確立した場所を記録する。signed_in_at は
-- サインインの時刻で、新規作成された行では created_at と一致するため、他のタイム
-- スタンプと同様 NOW() を既定値とし、INSERT では渡さない。
--
-- user_id の外部キーは ON DELETE CASCADE とする。セッションは独立したライフサイクルを
-- 持たず、自身の行の外に後始末すべきものもない純粋な従属データのため、ユーザーの
-- セッションはユーザーと一緒に消えるべきである。CASCADE なら将来の削除コードが先に
-- セッションを消し忘れても DB レベルで整合性が保証され、user_id インデックスにより
-- カスケードがテーブル全体を走査することも避けられる。
CREATE TABLE user_sessions (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token VARCHAR NOT NULL,
    ip_address VARCHAR NOT NULL,
    user_agent VARCHAR NOT NULL,
    signed_in_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (token)
);

CREATE INDEX ON user_sessions (user_id);

-- migrate:down

DROP TABLE IF EXISTS user_sessions;
