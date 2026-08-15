-- migrate:up

-- community_roles holds the roles a community defines for its members. A role
-- carries a name and nothing else: permission scopes are not modeled yet
-- because Groobb has no authorization gate that would consume them, and the
-- administrator role a community starts with is identified by that role's name.
-- Length limits are validated in the application, so name carries no length
-- modifier.
--
-- The community_id FK is ON DELETE CASCADE: a role belongs to exactly one
-- community, has no lifecycle of its own and nothing to clean up outside its
-- own row, so it must disappear with the community. CASCADE guarantees this at
-- the database level even if future deletion code forgets to remove the roles
-- first.
--
-- UNIQUE (community_id, name) keeps role names distinct within a community
-- while leaving two communities free to name a role the same. Having
-- community_id as its leading column, it doubles as the index that lists a
-- community's roles and that keeps the cascade from scanning the whole table.
--
-- UNIQUE (community_id, id) is redundant on its own, since id is already the
-- primary key. It exists so that the table assigning roles to members can carry
-- community_id in a composite foreign key: PostgreSQL only lets a foreign key
-- reference columns covered by a unique constraint. Routing that reference
-- through community_id is what makes the database able to reject assigning a
-- role to a member of another community.
--
-- [Ja] community_roles はコミュニティがメンバー向けに定義するロールを保持する。
-- ロールは名前だけを持ち、それ以外は持たない。権限スコープをまだモデル化しないのは、
-- Groobb にそれを消費する認可ゲートが無いためであり、コミュニティが最初から持つ
-- 管理者ロールはそのロール名で識別する。長さ制限はアプリ側でバリデーションするため、
-- name に長さ指定は付けない。
--
-- community_id の外部キーは ON DELETE CASCADE とする。ロールはちょうど 1 つの
-- コミュニティに属し、独立したライフサイクルを持たず、自身の行の外に後始末すべき
-- ものもないため、コミュニティと一緒に消えるべきである。CASCADE なら将来の削除
-- コードが先にロールを消し忘れても DB レベルで整合性が保証される。
--
-- UNIQUE (community_id, name) はコミュニティ内でロール名を一意に保ちつつ、異なる
-- コミュニティが同じロール名を付けることは許す。community_id が先頭カラムであるため、
-- コミュニティのロール一覧を引く索引と、カスケードがテーブル全体を走査するのを防ぐ
-- 索引を兼ねる。
--
-- UNIQUE (community_id, id) は id が既に主キーであるため、それ自体は冗長である。
-- これはロールをメンバーへ割り当てるテーブルが複合外部キーに community_id を含められる
-- ようにするために置く。PostgreSQL は一意制約で覆われたカラムの組しか外部キーの参照先に
-- できないためである。参照を community_id 経由にすることで初めて、別のコミュニティの
-- メンバーへのロール割当を DB が拒否できるようになる。
CREATE TABLE community_roles (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    community_id uuid NOT NULL REFERENCES communities (id) ON DELETE CASCADE,
    name VARCHAR NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (community_id, name),
    UNIQUE (community_id, id)
);

-- migrate:down

DROP TABLE IF EXISTS community_roles;
