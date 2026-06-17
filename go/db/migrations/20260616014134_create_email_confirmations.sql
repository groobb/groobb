-- migrate:up

-- email_confirmations holds the verification codes sent during flows that must
-- prove control of an email address (sign-up today; email change later). Each
-- row is one issued code: email is the address being verified, event names the
-- flow it belongs to, and code is the value the user types back. The row is
-- keyed by email rather than user_id because a confirmation is issued before the
-- user exists (sign-up verifies the address first, then creates the user), so
-- there is no user to reference yet. email is citext so it matches users.email
-- semantics (case-insensitive).
--
-- started_at is when the code was issued and is the basis for its expiry window;
-- for a freshly created row it equals created_at, so it defaults to NOW() like
-- the other timestamps and is not supplied on insert. succeeded_at is NULL until
-- the code is accepted, at which point the verification flow stamps it; the
-- verification queries and that update arrive with the consuming task.
--
-- [Ja] email_confirmations は、メールアドレスの管理権を証明する必要があるフロー
-- (現状はサインアップ、将来はメール変更) で送る確認コードを保持する。各行は発行された
-- 1 つのコードで、email は検証対象のアドレス、event はそれが属するフローの名前、code は
-- ユーザーが入力し返す値である。行は user_id ではなく email をキーとする。確認はユーザーが
-- 存在する前に発行される (サインアップはまずアドレスを検証し、その後ユーザーを作る) ため、
-- 参照すべきユーザーがまだ無いからである。email は citext とし、users.email と同じ意味論
-- (大文字小文字を区別しない) に揃える。
--
-- started_at はコードを発行した時刻で、有効期限ウィンドウの基準となる。新規作成された
-- 行では created_at と一致するため、他のタイムスタンプと同様 NOW() を既定値とし、INSERT
-- では渡さない。succeeded_at はコードが受理されるまで NULL で、受理時に検証フローが打刻
-- する。検証クエリとその更新は消費側タスクで追加する。
CREATE TABLE email_confirmations (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    email public.citext NOT NULL,
    event VARCHAR NOT NULL,
    code VARCHAR NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    succeeded_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- migrate:down

DROP TABLE IF EXISTS email_confirmations;
