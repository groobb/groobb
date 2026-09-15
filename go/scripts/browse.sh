#!/usr/bin/env bash
#
# browse.shはplaywright-cliを駆動してdevサイトのブラウザ確認を行う。
# Basic認証configの生成・単一ステップのdevサインイン・ログイン済み
# セッションでのスクショ・後片付けをまとめる。
#
# サインインに使う資格情報は、シードがアカウントを作成する元にしている名簿
# (go/seed-users.toml) から `groobb devcreds` を通して読む。ここでサインインする
# アカウントを、シードが実際に作成したアカウントそのものにするため。ベースURLは
# 引き続きKORYLUS_BROWSING_BASE_URLを前提とするため、op runラッパー配下
# (go/Makefileのbrowse-* ターゲット) から実行する。このURLをop run経由で読む
# ことで、.envをシェル評価して、そこに含まれるBasic認証credsを壊すのを避ける
# (credsが `$` を含む場合)。devサーバはTurnstileを無効化 (devの .envで
# GROOBB_TURNSTILE_DISABLE=true) して起動している必要があり、でないとBot検証で
# サインインの送信が弾かれる。
set -euo pipefail

SESSION=dev
# 役割の指定が無いときにサインインするアカウント。starterは掲示板に並ぶスレッドを
# 立てるアカウントで、そこが書いたものから辿れる画面が、もっともよく見られる画面になる。
DEFAULT_ROLE=starter
# Goモジュールのルート。本スクリプトの位置から求めることで、どのディレクトリから
# 実行してもヘルパー (groobb devcreds) を解決できるようにする。名簿を探す基準もここに
# なる。groobb devcredsが名簿のパスをモジュールルートからの相対で持っているため。
GO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR=/workspace/tmp
CONFIG_FILE="$TMP_DIR/browse-cli.config.json"
ORIGIN_FILE="$TMP_DIR/browse-cli.origin"
PROFILE_DIR="$TMP_DIR/browse-cli-profile"
SHOT_DIR="$TMP_DIR/browse"

pw() { playwright-cli -s="$SESSION" "$@"; }

# pw_checkedは、playwright-cliが終了コード0のまま出力へ記録する
# Playwrightのコマンドレベルエラーを検出する。呼び出し側は成功時の出力を
# 捨てられるが、エラーは常にstderrへ残す。
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

# build_configはKORYLUS_BROWSING_BASE_URLからBasic認証config
# (httpCredentials) を生成し、以降の遷移用にcredsを抜いたoriginファイルも書く。
# credsはhttpCredentials.originでそのoriginに固定し、他ホストの401
# (cross-originのサブリソースやリダイレクト) にdevのcredsが渡らないようにする。
#
# configはcredsを含むためgitignore済みtmpに0600で書き、ログインが
# ブラウザコンテキストに取り込んだ直後に削除する。書き込み前にunlinkするのは、
# mode引数がファイル新規作成時にしか効かず、既存ファイルへ上書きするとその
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

# cleanup_sessionは名前付きブラウザを閉じ、認証状態を削除する。ログインでは
# config生成の成功後だけEXIT trapとして設定し、それ以降の失敗で不完全な認証
# セッションがactiveなまま残らないようにする。
cleanup_session() {
  pw close >/dev/null 2>&1 || true
  rm -f "$CONFIG_FILE" "$ORIGIN_FILE"
  rm -rf "$PROFILE_DIR"
}

cmd_login() {
  local role="${1:-$DEFAULT_ROLE}"

  # 資格情報は、シードが読むのと同じ名簿 (go/seed-users.toml) から
  # `groobb devcreds` を通して受け取る。devcredsはメールアドレスとパスワードを1行ずつ
  # 出力する。シードが尋ねたファイルに尋ねることが、シード直後のサインインが通り続ける
  # 理由になる。片方だけを変えてもう片方が元のまま、ということが起こらないため。
  local credentials
  credentials="$(cd "$GO_DIR" && go run ./cmd/groobb devcreds "$role")"
  local lines=()
  mapfile -t lines <<<"$credentials"
  local email="${lines[0]:-}"
  local pass="${lines[1]:-}"
  if [ "${#lines[@]}" -ne 2 ] || [ -z "$email" ] || [ -z "$pass" ]; then
    echo "no credentials for role '$role': groobb devcreds must print exactly two non-empty lines" >&2
    exit 1
  fi

  # credsを含むconfigをどの終了経路でも削除し、ログイン途中の失敗
  # (下の明示rmへ到達する前のset -e abort) でもcredsをディスクに残さない。
  trap 'rm -f "$CONFIG_FILE"' EXIT

  build_config
  local origin
  origin="$(cat "$ORIGIN_FILE")"

  # ここから先の失敗では、credsを含むconfigだけでなくブラウザセッションも
  # 削除する。ログイン成功時は下でこのtrapを解除する。
  trap cleanup_session EXIT

  # Basic認証はconfig (httpCredentials) で渡す。永続プロファイルはログイン
  # Cookieをディスクに残し、起動中のセッションが別々のシェル呼び出しをまたいで
  # 生き続けられるようにする。
  pw_checked open "$origin/sign_in" --browser=chromium --persistent --profile="$PROFILE_DIR" --config="$CONFIG_FILE" >/dev/null

  # Groobbのサインインは単一ステップのフォーム (email + passwordを一括送信)。
  # emailは送信せずに入力し、passwordでEnterを押して送信する。Turnstileは
  # 無効化 (GROOBB_TURNSTILE_DISABLE=true) されている必要があり、でないと送信が
  # 弾かれる。name / attributeベースのロケータはラベル文言に依存せず、localeで
  # 変わらない。
  pw_checked fill 'input[name="email"]' "$email" >/dev/null
  pw_checked fill 'input[name="password"]' "$pass" --submit >/dev/null

  # コンテキストがcredsを保持したので、ディスク上のconfigはもう不要。
  # credsを残さないため削除する。
  rm -f "$CONFIG_FILE"

  # ログイン後のURLを報告する。サインイン成功時はホーム (/) へリダイレクト
  # する。/sign_in配下に留まる場合はサインインが完了していないことを意味する。
  # フォームの再描画 (422) か、2要素認証が有効なアカウントでTOTPチャレンジへ
  # 遷移したかのいずれかで、後者は本スクリプトが供給できないワンタイムコードを
  # 要求する。テストユーザーは2要素認証を無効にしたものを使う。pathnameは
  # `new URL()` ではなく正規表現で取り出す。playwright-cliのrun-codeはURL
  # コンストラクタが未定義のサンドボックスで動くため。ログイン可否はsentinelで
  # 返してbash側で判定し、失敗時に非ゼロ終了させる (run-code内のthrowは
  # エラーを表示するだけで終了コードは0になるため)。
  local result
  result="$(pw_checked --raw run-code "async page => {
    await page.waitForLoadState('networkidle');
    const href = page.url();
    const path = href.replace(/^[a-z][a-z0-9+.-]*:\/\/[^/]+/i, '').replace(/[?#].*/, '');
    const notSignedIn = path === '/sign_in' || path.startsWith('/sign_in/');
    return (notSignedIn ? 'NOT_SIGNED_IN ' : 'SIGNED_IN ') + href;
  }")"

  # --rawは返り値の文字列を二重引用符で囲むため、判定前に取り除く。
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
  echo "logged in as $role: ${result#SIGNED_IN }"
}

cmd_shot() {
  local path="${1:-/}"

  # originファイルの存在だけではセッションが使える証明にならない。ファイルは
  # hostとのbind mountである /workspace/tmpにありコンテナ再生成後も残るが、名前
  # 付きブラウザのデーモンは残らない。生存中のセッションをplaywright-cliに問い合わせ、
  # ファイルだけが残った状態でもログインへ誘導する。CLI自身の "run open first" 案内は
  # Basic認証とアプリのログインを飛ばしてしまうため。
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

  # 撮影前に同名の既存スクリーンショットを削除し、撮影失敗時に古い画像を
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

  # 実際に撮影したURLを報告する。同一originへのリダイレクト (セッション失効で
  # 保護ページが /sign_inへ飛ばされる場合など) は上のorigin判定を通るため、ファイル名
  # だけでは要求したpathを撮ったように見えてしまう。
  echo "screenshot: $filename ($actual_url)"
}

cmd_close() {
  cleanup_session
  echo "browser session closed and temp files removed"
}

case "${1:-}" in
  login)
    shift
    cmd_login "${1:-$DEFAULT_ROLE}"
    ;;
  shot)
    shift
    cmd_shot "${1:-/}"
    ;;
  close)
    cmd_close
    ;;
  *)
    echo "usage: browse.sh {login [role] | shot <path> | close}" >&2
    exit 2
    ;;
esac
