#!/bin/sh
# Stops what deploy/scripts/local-wsl.sh started (run as root inside the distro).
set -u
if pgrep -x blog-server >/dev/null 2>&1; then pkill -x blog-server && echo "[local-wsl] blog-server stopped"; else echo "[local-wsl] blog-server not running"; fi
if command -v nginx >/dev/null 2>&1; then nginx -s stop 2>/dev/null && echo "[local-wsl] nginx stopped" || echo "[local-wsl] nginx not running"; fi
