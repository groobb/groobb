-- migrate:up

-- Add deleted_at to users to mark an account as withdrawn (soft-deleted) without
-- physically removing the row yet. A withdrawal request sets deleted_at (and
-- anonymizes email/atname) synchronously so the account becomes inert
-- immediately, while the heavier physical DELETE of the row and its cascading
-- children is deferred to a periodic purge job. deleted_at is nullable because an
-- active account has no deletion time. The partial index over only the non-null
-- rows keeps the purge scan cheap: the vast majority of users stay active
-- (deleted_at IS NULL) and are excluded from the index.
--
-- [Ja] users に deleted_at を追加し、行をまだ物理削除せずにアカウントを退会済み
-- (論理削除) として印を付けられるようにする。退会リクエストは deleted_at のセット
-- (と email / atname の匿名化) を同期で行い、アカウントを即座に無効化する。一方で、
-- より重い行とその CASCADE する子データの物理 DELETE は定期パージジョブに先送りする。
-- deleted_at はアクティブなアカウントには削除時刻が無いため nullable とする。非 NULL 行
-- だけに張る部分インデックスはパージ用スキャンを安価に保つ。大多数のユーザーはアクティブ
-- (deleted_at IS NULL) なままでインデックスから除外されるためである。
ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX index_users_on_deleted_at ON users (deleted_at) WHERE deleted_at IS NOT NULL;

-- migrate:down

DROP INDEX IF EXISTS index_users_on_deleted_at;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
