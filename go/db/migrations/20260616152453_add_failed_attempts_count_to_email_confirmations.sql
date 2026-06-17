-- migrate:up

-- failed_attempts_count records how many times a wrong code was submitted for
-- this confirmation. The verification flow increments it on each mismatch and
-- treats a confirmation as no longer active once it reaches the limit (5), so a
-- leaked handoff id cannot be used to brute-force the 6-digit code within the
-- 15-minute window. The limit itself lives in the "active" lookup query
-- (failed_attempts_count < 5), not in this column; the column only holds the
-- count. It defaults to 0 and is never NULL, so existing-row and insert paths
-- need no special handling.
--
-- [Ja] failed_attempts_count はこの確認に対して誤ったコードが送信された回数を記録する。
-- 検証フローは不一致のたびにこれをインクリメントし、上限 (5) に達した確認は active で
-- なくなったものとして扱う。これにより、漏れた受け渡し id を使って 15 分のウィンドウ内に
-- 6 桁コードを総当たりすることを防ぐ。上限そのものは "active" を引くクエリ側
-- (failed_attempts_count < 5) にあり、この列は回数を保持するだけである。既定値は 0 で
-- NULL を取らないため、既存行・INSERT 経路とも特別な扱いは要らない。
ALTER TABLE email_confirmations
    ADD COLUMN failed_attempts_count INT NOT NULL DEFAULT 0;

-- migrate:down

ALTER TABLE email_confirmations
    DROP COLUMN failed_attempts_count;
