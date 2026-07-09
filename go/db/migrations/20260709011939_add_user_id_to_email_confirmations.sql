-- migrate:up

-- Add a nullable user_id so an email_confirmations row can be tied to the
-- authenticated user who requested it. Sign-up keeps issuing confirmations with
-- user_id NULL: the address is verified before the user exists, so there is no
-- user to reference yet. Email change is different: it is requested by a
-- logged-in user, so the confirmation is tied to that user at the database level,
-- which lets the confirm step look up "this user's pending email change" straight
-- from the session's user_id instead of relying on a handoff cookie.
--
-- The FK is ON DELETE CASCADE: a confirmation is pure dependent data with no
-- independent lifecycle and nothing to clean up outside its own row, so it must
-- disappear with its user. CASCADE guarantees this at the database level even if
-- future deletion code forgets to remove the confirmation first. The index on
-- user_id serves the by-user lookup above and keeps the cascade from scanning the
-- whole table.
--
-- [Ja] email_confirmations の行を、それを申請した認証済みユーザーに紐付けられるよう、
-- nullable な user_id を追加する。サインアップは引き続き user_id を NULL のまま確認を
-- 発行する。アドレスの検証はユーザーが存在する前に行われるため、参照すべきユーザーが
-- まだ無いからである。メール変更は異なり、ログイン済みユーザーが申請するため、確認を
-- DB レベルでそのユーザーに紐付ける。これにより確認ステップは handoff Cookie に頼らず、
-- セッションの user_id から「このユーザーの保留中のメール変更」を直接引ける。
--
-- 外部キーは ON DELETE CASCADE とする。確認は独立したライフサイクルを持たず、自身の行の
-- 外に後始末すべきものもない純粋な従属データのため、ユーザーと一緒に消えるべきである。
-- CASCADE なら将来の削除コードが先に確認を消し忘れても DB レベルで整合性が保証される。
-- user_id のインデックスは上記のユーザー単位のルックアップに使われ、カスケードがテーブル
-- 全体を走査することも避ける。
ALTER TABLE email_confirmations
    ADD COLUMN user_id uuid REFERENCES users (id) ON DELETE CASCADE;

CREATE INDEX index_email_confirmations_on_user_id ON email_confirmations (user_id);

-- migrate:down

ALTER TABLE email_confirmations
    DROP COLUMN user_id;
