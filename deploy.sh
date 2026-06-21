#!/usr/bin/env bash
set -euo pipefail

# grid-guard build + deploy helper.
#   ./deploy.sh build    -> build the static linux binary into ./bin/
#   ./deploy.sh push     -> build + scp the binary & deploy assets to the host,
#                           then print the remote install commands
# Build target defaults to linux/amd64; override via GOOS/GOARCH (e.g. GOARCH=arm64).
# Deploy target overridable via env: DEPLOY_TARGET, DEPLOY_PATH.

HERE="$(cd "$(dirname "$0")" && pwd)"
BIN_NAME="grid-guard"
PREFIX_BIN="/usr/local/bin"
CONF_DIR="/etc/grid-guard"
DEPLOY_TARGET="${DEPLOY_TARGET:-grid-guard-oracle}"
DEPLOY_PATH="${DEPLOY_PATH:-/home/opc/grid-guard}"

build() {
  local goos="${GOOS:-linux}" goarch="${GOARCH:-amd64}"
  mkdir -p "$HERE/bin"
  echo ">> building $BIN_NAME ($goos/$goarch)"
  ( cd "$HERE" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags "-s -w" -o "bin/$BIN_NAME" . )
  ls -lh "$HERE/bin/$BIN_NAME"
  if [ "$goos" = "linux" ]; then
    if file "$HERE/bin/$BIN_NAME" | grep -q "statically linked"; then
      echo ">> OK: statically linked linux binary (no runtime deps)"
    else
      echo "!! WARNING: binary not detected as statically linked"
    fi
  fi
}

# push builds the linux binary and copies it + deploy assets to the host.
# Defaults to amd64; set GOARCH=arm64 for ARM hosts. It does NOT auto-enable
# anything remotely; it prints the remote install commands.
push() {
  GOOS=linux GOARCH="${GOARCH:-amd64}" build
  echo ">> copying to $DEPLOY_TARGET:$DEPLOY_PATH"
  ssh "$DEPLOY_TARGET" "mkdir -p $DEPLOY_PATH/deploy"
  scp "$HERE/bin/$BIN_NAME" "$DEPLOY_TARGET:$DEPLOY_PATH/$BIN_NAME"
  scp "$HERE"/deploy/grid-guard.service "$HERE"/deploy/grid-guard.timer \
      "$HERE"/deploy/config.example.json "$HERE"/deploy/crontab.example \
      "$DEPLOY_TARGET:$DEPLOY_PATH/deploy/"
  echo ">> on the host, finish install with:"
  echo "   ssh $DEPLOY_TARGET"
  echo "   sudo install -m755 $DEPLOY_PATH/$BIN_NAME $PREFIX_BIN/$BIN_NAME"
  echo "   sudo mkdir -p $CONF_DIR && sudo install -m600 $DEPLOY_PATH/deploy/config.example.json $CONF_DIR/config.json"
  echo "   sudo nano $CONF_DIR/config.json   # fill in real secrets"
  echo "   sudo install -m644 $DEPLOY_PATH/deploy/grid-guard.{service,timer} /etc/systemd/system/"
  echo "   sudo systemctl daemon-reload && sudo systemctl enable --now grid-guard.timer"
}

case "${1:-}" in
  build) build ;;
  push) push ;;
  *) echo "usage: $0 {build|push}"; exit 1 ;;
esac
