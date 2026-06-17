-- migrate:up

-- password_reset_tokens holds the one-time tokens issued when a user asks to
-- reset their password. The plaintext token is mailed to the user inside a reset
-- link and never stored; only its SHA-256 hash is kept in token_digest, so a
-- database leak does not expose usable tokens. token_digest is UNIQUE because the
-- reset link is looked up by digest (the lookup query arrives with the consuming
-- task). expires_at bounds how long the link works, and used_at is stamped when
-- the token is spent so a link cannot be replayed (both the expiry check and that
-- stamp arrive with the consuming task).
--
-- The user_id FK is ON DELETE CASCADE: a reset token is pure dependent data with
-- no independent lifecycle and nothing to clean up outside its own row, so it
-- must disappear with the user. CASCADE guarantees this at the database level
-- even if future deletion code forgets to remove the token first; the index on
-- user_id (which also serves deleting a user's outstanding tokens when a new one
-- is issued) keeps the cascade from scanning the whole table.
--
-- [Ja] password_reset_tokens は、ユーザーがパスワードのリセットを申請したときに発行する
-- 使い捨てトークンを保持する。平文トークンはリセットリンクに入れてユーザーへメールし、
-- 保存はしない。SHA-256 ハッシュだけを token_digest に持つため、DB が漏えいしても使える
-- トークンは露出しない。token_digest はリセットリンクがダイジェストで引かれる (引くクエリは
-- 消費側タスクで追加) ため UNIQUE とする。expires_at はリンクが有効な期間を区切り、used_at は
-- トークンが使われたときに打刻してリンクの再利用を防ぐ (有効期限チェックとこの打刻は消費側
-- タスクで追加)。
--
-- user_id の外部キーは ON DELETE CASCADE とする。リセットトークンは独立したライフサイクルを
-- 持たず、自身の行の外に後始末すべきものもない純粋な従属データのため、ユーザーと一緒に
-- 消えるべきである。CASCADE なら将来の削除コードが先にトークンを消し忘れても DB レベルで
-- 整合性が保証され、user_id のインデックス (新しいトークン発行時に未使用トークンを削除する
-- 用途も兼ねる) によりカスケードがテーブル全体を走査することも避けられる。
CREATE TABLE password_reset_tokens (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_digest VARCHAR NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (token_digest)
);

CREATE INDEX index_password_reset_tokens_on_user_id ON password_reset_tokens (user_id);

-- migrate:down

DROP TABLE IF EXISTS password_reset_tokens;
