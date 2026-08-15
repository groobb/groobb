-- migrate:up

-- community_member_roles assigns roles to members, many to many: a member may
-- hold several roles and a role may be held by several members. A community
-- starts with one row here, giving its creator the administrator role.
--
-- community_id duplicates what community_member_id already determines, and is
-- kept as the carrier of an integrity constraint. Two single-column foreign keys
-- (one to community_members, one to community_roles) cannot stop a row from
-- pairing a member of one community with a role of another. Sharing community_id
-- across both composite foreign keys makes the database enforce that the member's
-- community and the role's community are the same value, so that mismatch cannot
-- be represented at all.
--
-- No foreign key points at communities directly. community_members.community_id
-- and community_roles.community_id each reference communities (id), so the link
-- holds transitively through the two composite keys above and a community's
-- deletion cascades along both paths.
--
-- Both composite foreign keys are ON DELETE CASCADE: an assignment is a pure
-- join row with no lifecycle of its own and nothing to clean up outside itself,
-- so removing a member or a role must remove the assignments that reference it.
--
-- UNIQUE (community_member_id, community_role_id) keeps the same role from being
-- assigned to the same member twice.
--
-- The two indexes cover the composite foreign keys, so deleting a member or a
-- role does not scan the whole table.
-- index_community_member_roles_on_community_and_member also serves the lookup of
-- the roles a member holds.
--
-- [Ja] community_member_roles はロールをメンバーへ多対多で割り当てる。1 人のメンバーが
-- 複数のロールを持てて、1 つのロールを複数のメンバーが持てる。コミュニティは作成時に
-- ここへ 1 行を持ち、作成者へ管理者ロールを与える。
--
-- community_id は community_member_id から既に定まる冗長な列だが、整合性制約の担い手
-- として持たせる。単一列の外部キー 2 本 (community_members へのものと community_roles
-- へのもの) では、あるコミュニティのメンバーと別のコミュニティのロールを組み合わせた行を
-- 拒否できない。2 本の複合外部キーで community_id を共有させると、メンバー側の
-- community_id とロール側の community_id が同じ値であることを DB が強制するため、この
-- 不整合はそもそも表現できなくなる。
--
-- communities への直接の外部キーは張らない。community_members.community_id と
-- community_roles.community_id がそれぞれ communities (id) を参照しているため、上記 2 本の
-- 複合キー経由で推移的に関係が保たれ、コミュニティ削除時のカスケードもこの 2 経路で届く。
--
-- 2 本の複合外部キーはいずれも ON DELETE CASCADE とする。割当は独立したライフサイクルを
-- 持たず自身の外に後始末すべきものもない純粋な中間行のため、メンバーやロールを消すときは
-- それを参照する割当も消えなければならない。
--
-- UNIQUE (community_member_id, community_role_id) は同じロールを同じメンバーへ二重に
-- 割り当てることを防ぐ。
--
-- 2 つの索引は複合外部キーを覆い、メンバーやロールの削除時にテーブル全体を走査しない
-- ようにする。index_community_member_roles_on_community_and_member は「あるメンバーが
-- 持つロール」の引き当てにも効く。
CREATE TABLE community_member_roles (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    community_id uuid NOT NULL,
    community_member_id uuid NOT NULL,
    community_role_id uuid NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (community_id, community_member_id) REFERENCES community_members (community_id, id) ON DELETE CASCADE,
    FOREIGN KEY (community_id, community_role_id) REFERENCES community_roles (community_id, id) ON DELETE CASCADE,
    UNIQUE (community_member_id, community_role_id)
);

CREATE INDEX index_community_member_roles_on_community_and_member ON community_member_roles (community_id, community_member_id);

CREATE INDEX index_community_member_roles_on_community_and_role ON community_member_roles (community_id, community_role_id);

-- migrate:down

DROP INDEX IF EXISTS index_community_member_roles_on_community_and_role;

DROP INDEX IF EXISTS index_community_member_roles_on_community_and_member;

DROP TABLE IF EXISTS community_member_roles;
