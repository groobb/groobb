-- migrate:up

-- pgcrypto provides gen_random_bytes(), used by generate_ulid() below. citext
-- provides case-insensitive text columns (e.g. users.email), so uniqueness and
-- lookups ignore letter case without normalizing in the application.
--
-- [Ja] pgcrypto は下の generate_ulid() が使う gen_random_bytes() を提供する。
-- citext は大文字小文字を区別しないテキスト列 (例: users.email) を提供し、
-- アプリ側で正規化せずとも一意性・検索が大文字小文字を無視するようにする。
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;

-- generate_ulid() returns a ULID rendered as a uuid: a 48-bit millisecond
-- timestamp prefix followed by 80 random bits. Because it is time-ordered, it
-- keeps primary-key index inserts append-mostly (unlike fully random UUIDv4),
-- while staying byte-compatible with the uuid type so Go can treat it as
-- uuid.UUID. This matches the established sister-project pattern.
--
-- [Ja] generate_ulid() は ULID を uuid として返す。先頭 48 bit がミリ秒
-- タイムスタンプ、続く 80 bit が乱数。時刻順のため主キーのインデックス挿入が
-- ほぼ末尾追記になり (完全乱数の UUIDv4 と異なり) 断片化しにくく、かつ uuid 型と
-- バイト互換なので Go 側は uuid.UUID として扱える。姉妹プロジェクトの確立パターンに
-- 揃えている。
CREATE FUNCTION public.generate_ulid() RETURNS uuid
    LANGUAGE sql
    AS $$
  SELECT (lpad(to_hex(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint), 12, '0') || encode(gen_random_bytes(10), 'hex'))::uuid;
$$;

-- migrate:down

DROP FUNCTION IF EXISTS public.generate_ulid();
DROP EXTENSION IF EXISTS citext;
DROP EXTENSION IF EXISTS pgcrypto;
