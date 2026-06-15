-- migrate:up

-- users is the global, canonical identity / authentication anchor. It holds
-- only identity-level attributes (email, locale, time_zone); presentation and
-- role attributes (atname, display name, role) live on space_members in a later
-- plan, and the password digest is split into a separate credentials table.
-- email is citext + UNIQUE so a single verified address maps to exactly one
-- account regardless of letter case, which the federated identity model relies
-- on.
--
-- [Ja] users はグローバルで正準な身元 / 認証アンカー。身元レベルの属性
-- (email / locale / time_zone) のみを持つ。表示・権限の属性 (atname / 表示名 /
-- ロール) は後続計画で space_members に置き、パスワードダイジェストは別の資格情報
-- テーブルに分離する。email は citext + UNIQUE とし、大文字小文字によらず 1 つの
-- 検証済みアドレスが 1 アカウントに対応する。これは連合型アイデンティティモデルの
-- 前提となる。
CREATE TABLE users (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    email public.citext NOT NULL,
    locale VARCHAR NOT NULL,
    time_zone VARCHAR NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (email)
);

-- migrate:down

DROP TABLE IF EXISTS users;
