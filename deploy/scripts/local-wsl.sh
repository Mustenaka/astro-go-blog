#!/bin/sh
# Local Linux run inside WSL (run as root inside the distro):
#   wsl -d Ubuntu -u root -- sh /mnt/d/Work/blog/newsite/deploy/scripts/local-wsl.sh
#
# Takes the artifacts built on Windows (site/dist, api/bin/blog-server-linux, api/data) and runs
# them the way the server will: Nginx serves the static site and forwards /api/v1/ to the Go
# process; the Go process listens on 127.0.0.1:8080 and also serves /assets/ from local storage.
# Nothing here touches the repository; everything lives under /srv/blog.
set -eu

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
DEST="${DEST:-/srv/blog}"
PORT="${PORT:-8088}"

say() { printf '\033[1;36m[local-wsl]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[local-wsl] %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" = 0 ] || fail "run as root: wsl -d <distro> -u root -- sh $0"
[ -f "$REPO/site/dist/index.html" ] || fail "site/dist missing: run 'pnpm -C site build' on Windows first"
[ -f "$REPO/api/bin/blog-server-linux" ] || fail "api/bin/blog-server-linux missing: cross-compile on Windows first"
[ -f "$REPO/api/.env" ] || fail "api/.env missing (see docs/runbooks/local-dev.md)"

# 1. nginx
if ! command -v nginx >/dev/null 2>&1; then
  say "installing nginx (apt)"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq || fail "apt-get update failed: check the network/proxy (see .wslconfig mirrored mode)"
  apt-get install -y -qq nginx >/dev/null || fail "apt-get install nginx failed"
fi

# 2. artifacts
say "copying artifacts to $DEST"
mkdir -p "$DEST/site" "$DEST/bin" "$DEST/data"
rm -rf "$DEST/site"/*
cp -r "$REPO/site/dist/." "$DEST/site/"
cp "$REPO/api/bin/blog-server-linux" "$DEST/bin/blog-server"
chmod +x "$DEST/bin/blog-server"
if [ -d "$REPO/api/data/assets" ]; then
  cp -r "$REPO/api/data/assets" "$DEST/data/"
fi
if [ -f "$REPO/api/data/blog.db" ] && [ ! -f "$DEST/data/blog.db" ]; then
  cp "$REPO/api/data/blog.db" "$DEST/data/blog.db"
fi

# 3. .env: start from the Windows api/.env, override what differs on Linux
say "writing $DEST/.env"
grep -vE '^(DATA_DIR|RELEASE_SCRIPT|CONTENT_DIR|SITE_DIR|CORS_ORIGINS|ADDR)=' "$REPO/api/.env" > "$DEST/.env"
cat >> "$DEST/.env" <<EOF
ADDR=127.0.0.1:8080
DATA_DIR=$DEST/data
CONTENT_DIR=$REPO/content
SITE_DIR=$REPO/site
RELEASE_SCRIPT=$DEST/release.sh
CORS_ORIGINS=http://localhost:$PORT,http://127.0.0.1:$PORT
EOF
cat > "$DEST/release.sh" <<'EOF'
#!/bin/sh
# Local WSL run: the static site is built on Windows (pnpm -C site build) and copied over by
# deploy/scripts/local-wsl.sh. Re-run that script to publish a new build.
echo "[release] local WSL run: build is produced on Windows; re-run deploy/scripts/local-wsl.sh to publish"
exit 0
EOF
chmod +x "$DEST/release.sh"

# 4. nginx site
say "configuring nginx on port $PORT"
sed -e "s|__LISTEN__|$PORT|g" -e "s|__SERVER_NAME__|localhost|g" -e "s|__SITE_ROOT__|$DEST/site|g" \
  "$REPO/deploy/nginx/blog.conf" > /etc/nginx/sites-available/blog
ln -sf /etc/nginx/sites-available/blog /etc/nginx/sites-enabled/blog
rm -f /etc/nginx/sites-enabled/default
nginx -t 2>&1 | tail -1
if nginx -s reload 2>/dev/null; then say "nginx reloaded"; else nginx && say "nginx started"; fi

# 5. Go backend
if pgrep -x blog-server >/dev/null 2>&1; then
  say "stopping previous blog-server"
  pkill -x blog-server || true
  sleep 1
fi
say "starting blog-server (log: $DEST/api.log)"
cd "$DEST"
setsid nohup "$DEST/bin/blog-server" > "$DEST/api.log" 2>&1 &
sleep 2

# 6. checks
code() { curl -s -o /dev/null -w '%{http_code}' -m 5 "$1" 2>/dev/null || echo 000; }
say "healthz        : $(code http://127.0.0.1:8080/healthz)"
say "home           : $(code http://127.0.0.1:$PORT/)"
say "post           : $(code http://127.0.0.1:$PORT/posts/hello-world/)"
say "api counters   : $(code "http://127.0.0.1:$PORT/api/v1/counters?slugs=hello-world")"
say "admin blocked  : $(code http://127.0.0.1:$PORT/admin/) (expect 404)"
say "legacy 301     : $(code http://127.0.0.1:$PORT/index.php/blog/) (expect 301)"
say "legacy 410     : $(code http://127.0.0.1:$PORT/index.php/download/) (expect 410)"
say "done. open http://localhost:$PORT/ on Windows; admin at http://localhost:8080/admin/"
