#!/usr/bin/env bash
#
# browse.sh drives playwright-cli for browser verification of the dev site:
# it generates the Basic-auth config, runs the single-step dev sign-in, reuses
# the logged-in session for screenshots, and cleans up.
#
# It expects KORYLUS_BROWSING_* in the environment, so run it under the op-run
# wrapper (see the browse-* targets in go/Makefile). Reading credentials through
# op-run avoids evaluating the .env in a shell, which would corrupt any
# credential containing a `$`. The dev server must run with Turnstile disabled
# (GROOBB_TURNSTILE_DISABLE=true in the dev .env) or bot verification blocks the
# sign-in submit.
#
# [Ja] browse.sh は playwright-cli を駆動して dev サイトのブラウザ確認を行う。
# Basic 認証 config の生成・単一ステップの dev サインイン・ログイン済み
# セッションでのスクショ・後片付けをまとめる。
#
# KORYLUS_BROWSING_* が環境にある前提なので、op run ラッパー配下 (go/Makefile の
# browse-* ターゲット) から実行する。creds を op run 経由で読むことで、.env を
# シェル評価して `$` を含む creds を壊すのを避ける。dev サーバは Turnstile を
# 無効化 (dev の .env で GROOBB_TURNSTILE_DISABLE=true) して起動している必要が
# あり、でないと Bot 検証でサインインの送信が弾かれる。
set -euo pipefail

SESSION=dev
TMP_DIR=/workspace/tmp
CONFIG_FILE="$TMP_DIR/browse-cli.config.json"
ORIGIN_FILE="$TMP_DIR/browse-cli.origin"
PROFILE_DIR="$TMP_DIR/browse-cli-profile"
SHOT_DIR="$TMP_DIR/browse"

pw() { playwright-cli -s="$SESSION" "$@"; }

# pw_checked handles command-level Playwright errors, which playwright-cli
# reports in its output while still exiting with status 0. Callers may discard
# successful output, but errors are always preserved on stderr.
#
# [Ja] pw_checked は、playwright-cli が終了コード 0 のまま出力へ記録する
# Playwright のコマンドレベルエラーを検出する。呼び出し側は成功時の出力を
# 捨てられるが、エラーは常に stderr へ残す。
pw_checked() {
  local output
  if ! output="$(pw "$@" 2>&1)"; then
    printf '%s\n' "$output" >&2
    return 1
  fi
  if [[ "$output" == *"### Error"* ]]; then
    printf '%s\n' "$output" >&2
    return 1
  fi
  printf '%s\n' "$output"
}

# build_config writes the Basic-auth config (httpCredentials) parsed from
# KORYLUS_BROWSING_BASE_URL, plus a credential-free origin file for later
# navigation. The credentials are pinned to that origin
# (httpCredentials.origin), so a 401 from any other host — a cross-origin
# subresource or a redirect — never receives the dev credentials.
#
# The config carries the credentials, so it is written 0600 under the gitignored
# tmp dir and removed as soon as login captures it in the browser context. It is
# unlinked before the write because the mode argument only applies when the file
# is created: writing over an existing file would keep that file's permissions.
#
# [Ja] build_config は KORYLUS_BROWSING_BASE_URL から Basic 認証 config
# (httpCredentials) を生成し、以降の遷移用に creds を抜いた origin ファイルも書く。
# creds は httpCredentials.origin でその origin に固定し、他ホストの 401
# (cross-origin のサブリソースやリダイレクト) に dev の creds が渡らないようにする。
#
# config は creds を含むため gitignore 済み tmp に 0600 で書き、ログインが
# ブラウザコンテキストに取り込んだ直後に削除する。書き込み前に unlink するのは、
# mode 引数がファイル新規作成時にしか効かず、既存ファイルへ上書きするとその
# ファイルの権限がそのまま残るため。
build_config() {
  mkdir -p "$TMP_DIR"
  node -e '
    const fs = require("fs");
    const raw = process.env.KORYLUS_BROWSING_BASE_URL || "";
    if (!raw) { console.error("KORYLUS_BROWSING_BASE_URL is not set"); process.exit(1); }
    const u = new URL(raw);
    const origin = u.origin;
    const cfg = { browser: { contextOptions: { httpCredentials: {
      username: decodeURIComponent(u.username),
      password: decodeURIComponent(u.password),
      origin,
    } } } };
    fs.rmSync(process.argv[1], { force: true });
    fs.writeFileSync(process.argv[1], JSON.stringify(cfg), { mode: 0o600 });
    fs.writeFileSync(process.argv[2], origin);
  ' "$CONFIG_FILE" "$ORIGIN_FILE"
}

# cleanup_session closes the named browser and removes its authentication state.
# Login installs it as an EXIT trap only after config generation has succeeded,
# so any later failure cannot leave a partially authenticated session active.
#
# [Ja] cleanup_session は名前付きブラウザを閉じ、認証状態を削除する。ログインでは
# config 生成の成功後だけ EXIT trap として設定し、それ以降の失敗で不完全な認証
# セッションが active なまま残らないようにする。
cleanup_session() {
  pw close >/dev/null 2>&1 || true
  rm -f "$CONFIG_FILE" "$ORIGIN_FILE"
  rm -rf "$PROFILE_DIR"
}

cmd_login() {
  local n="${1:-1}"
  local email_var="KORYLUS_BROWSING_USER${n}_EMAIL"
  local pass_var="KORYLUS_BROWSING_USER${n}_PASSWORD"
  local email="${!email_var:-}"
  local pass="${!pass_var:-}"
  if [ -z "$email" ] || [ -z "$pass" ]; then
    echo "USER${n} credentials are not set (${email_var} / ${pass_var})" >&2
    exit 1
  fi

  # Remove the credential-bearing config on any exit, so a mid-login failure
  # (a set -e abort before the explicit rm below) never leaves credentials at
  # rest.
  #
  # [Ja] creds を含む config をどの終了経路でも削除し、ログイン途中の失敗
  # (下の明示 rm へ到達する前の set -e abort) でも creds をディスクに残さない。
  trap 'rm -f "$CONFIG_FILE"' EXIT

  build_config
  local origin
  origin="$(cat "$ORIGIN_FILE")"

  # From this point on, a failure must remove the browser session as well as the
  # credential-bearing config. A successful login clears this trap below.
  #
  # [Ja] ここから先の失敗では、creds を含む config だけでなくブラウザセッションも
  # 削除する。ログイン成功時は下でこの trap を解除する。
  trap cleanup_session EXIT

  # Basic auth is passed via the config (httpCredentials); the persistent
  # profile keeps the login cookies on disk so a still-running session survives
  # across separate shell invocations.
  #
  # [Ja] Basic 認証は config (httpCredentials) で渡す。永続プロファイルはログイン
  # Cookie をディスクに残し、起動中のセッションが別々のシェル呼び出しをまたいで
  # 生き続けられるようにする。
  pw_checked open "$origin/sign_in" --browser=chromium --persistent --profile="$PROFILE_DIR" --config="$CONFIG_FILE" >/dev/null

  # Groobb's sign-in is a single-step form (email + password submitted together).
  # Fill email without submitting, then submit with Enter on the password field.
  # Turnstile must be disabled (GROOBB_TURNSTILE_DISABLE=true) or the submit is
  # blocked. The name/attribute locators avoid depending on label text, which
  # changes with the locale.
  #
  # [Ja] Groobb のサインインは単一ステップのフォーム (email + password を一括送信)。
  # email は送信せずに入力し、password で Enter を押して送信する。Turnstile は
  # 無効化 (GROOBB_TURNSTILE_DISABLE=true) されている必要があり、でないと送信が
  # 弾かれる。name / attribute ベースのロケータはラベル文言に依存せず、locale で
  # 変わらない。
  pw_checked fill 'input[name="email"]' "$email" >/dev/null
  pw_checked fill 'input[name="password"]' "$pass" --submit >/dev/null

  # The context now holds the credentials, so the on-disk config is no longer
  # needed; drop it to avoid leaving credentials at rest.
  #
  # [Ja] コンテキストが creds を保持したので、ディスク上の config はもう不要。
  # creds を残さないため削除する。
  rm -f "$CONFIG_FILE"

  # Report the post-login URL. A successful sign-in redirects to home (/);
  # staying under /sign_in means the sign-in did not complete — either the form
  # was re-rendered (422) or the account has two-factor auth enabled and landed
  # on the TOTP challenge, which needs a one-time code this script cannot
  # supply. Use test users without two-factor auth. The pathname is extracted
  # with a regex rather than `new URL()`: playwright-cli's run-code runs in a
  # sandbox where the URL constructor is not defined. The signed-in decision is
  # returned as a sentinel and acted on in bash so a failed login exits non-zero
  # (a throw inside run-code only prints an error and exits 0).
  #
  # [Ja] ログイン後の URL を報告する。サインイン成功時はホーム (/) へリダイレクト
  # する。/sign_in 配下に留まる場合はサインインが完了していないことを意味する。
  # フォームの再描画 (422) か、2 要素認証が有効なアカウントで TOTP チャレンジへ
  # 遷移したかのいずれかで、後者は本スクリプトが供給できないワンタイムコードを
  # 要求する。テストユーザーは 2 要素認証を無効にしたものを使う。pathname は
  # `new URL()` ではなく正規表現で取り出す。playwright-cli の run-code は URL
  # コンストラクタが未定義のサンドボックスで動くため。ログイン可否は sentinel で
  # 返して bash 側で判定し、失敗時に非ゼロ終了させる (run-code 内の throw は
  # エラーを表示するだけで終了コードは 0 になるため)。
  local result
  result="$(pw_checked --raw run-code "async page => {
    await page.waitForLoadState('networkidle');
    const href = page.url();
    const path = href.replace(/^[a-z][a-z0-9+.-]*:\/\/[^/]+/i, '').replace(/[?#].*/, '');
    const notSignedIn = path === '/sign_in' || path.startsWith('/sign_in/');
    return (notSignedIn ? 'NOT_SIGNED_IN ' : 'SIGNED_IN ') + href;
  }")"

  # --raw wraps the returned string in double quotes; strip them before matching.
  #
  # [Ja] --raw は返り値の文字列を二重引用符で囲むため、判定前に取り除く。
  result="${result%\"}"
  result="${result#\"}"

  if [[ "$result" == NOT_SIGNED_IN* ]]; then
    echo "sign-in did not complete (still under /sign_in): ${result#NOT_SIGNED_IN }" >&2
    exit 1
  fi
  if [[ "$result" != "SIGNED_IN $origin" && "$result" != "SIGNED_IN $origin/"* ]]; then
    echo "could not verify sign-in at the expected origin: $result" >&2
    exit 1
  fi
  trap - EXIT
  echo "logged in as USER${n}: ${result#SIGNED_IN }"
}

cmd_shot() {
  local path="${1:-/}"

  # The origin file alone does not prove the session is usable: it lives under
  # /workspace/tmp, a host bind mount that survives a container rebuild, while
  # the named browser daemon does not. Ask playwright-cli which sessions are
  # live so a stale file still routes the caller to login, rather than to the
  # CLI's own "run open first" hint, which skips Basic auth and the app login.
  #
  # [Ja] origin ファイルの存在だけではセッションが使える証明にならない。ファイルは
  # host との bind mount である /workspace/tmp にありコンテナ再生成後も残るが、名前
  # 付きブラウザのデーモンは残らない。生存中のセッションを playwright-cli に問い合わせ、
  # ファイルだけが残った状態でもログインへ誘導する。CLI 自身の "run open first" 案内は
  # Basic 認証とアプリのログインを飛ばしてしまうため。
  local sessions
  sessions="$(playwright-cli list 2>&1 || true)"
  if [ ! -f "$ORIGIN_FILE" ] || [[ "$sessions" != *"- $SESSION:"* ]]; then
    echo "no active session; run 'make browse-login' first" >&2
    exit 1
  fi
  mkdir -p "$SHOT_DIR"
  local origin
  origin="$(cat "$ORIGIN_FILE")"
  local name
  name="$(printf '%s' "$path" | sed 's#[^a-zA-Z0-9]#_#g; s#^_*##')"
  [ -n "$name" ] || name=home
  local filename="$SHOT_DIR/$name.png"

  # Remove a previous screenshot before navigating so a failed capture can
  # never be mistaken for a fresh result.
  #
  # [Ja] 撮影前に同名の既存スクリーンショットを削除し、撮影失敗時に古い画像を
  # 新しい結果と誤認できないようにする。
  rm -f "$filename"

  pw_checked goto "$origin$path" >/dev/null

  local actual_url
  actual_url="$(pw_checked --raw run-code "async page => {
    await page.waitForLoadState('networkidle');
    return page.url();
  }")"
  actual_url="${actual_url%\"}"
  actual_url="${actual_url#\"}"
  if [[ "$actual_url" != "$origin" && "$actual_url" != "$origin/"* ]]; then
    echo "page left the expected origin: $actual_url" >&2
    exit 1
  fi

  pw_checked screenshot --filename="$filename" >/dev/null
  if [ ! -s "$filename" ]; then
    echo "screenshot was not created: $filename" >&2
    exit 1
  fi

  # Report the URL that was actually captured. A same-origin redirect (an
  # expired session bouncing a protected page to /sign_in) passes the origin
  # check above, so the filename on its own would suggest the requested path
  # was captured.
  #
  # [Ja] 実際に撮影した URL を報告する。同一 origin へのリダイレクト (セッション失効で
  # 保護ページが /sign_in へ飛ばされる場合など) は上の origin 判定を通るため、ファイル名
  # だけでは要求した path を撮ったように見えてしまう。
  echo "screenshot: $filename ($actual_url)"
}

cmd_close() {
  cleanup_session
  echo "browser session closed and temp files removed"
}

case "${1:-}" in
  login)
    shift
    cmd_login "${1:-1}"
    ;;
  shot)
    shift
    cmd_shot "${1:-/}"
    ;;
  close)
    cmd_close
    ;;
  *)
    echo "usage: browse.sh {login [user_number] | shot <path> | close}" >&2
    exit 2
    ;;
esac
