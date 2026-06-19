#!/bin/sh
set -eu

BIN_NAME=mihoro
REPO="aceak/mihoro-go"
VERSION="${VERSION:-latest}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
MIRROR="${MIHORO_GITHUB_MIRROR:-}"

# Parse --mirror flag
while [ $# -gt 0 ]; do
    case "$1" in
        --mirror) MIRROR="$2"; shift 2 ;;
        *) echo "Unknown option: $1" && exit 1 ;;
    esac
done

# Detect arch
case "$(uname -m)" in
    x86_64|amd64)  ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    armv7l)        ARCH=armv7 ;;
    riscv64)       ARCH=riscv64 ;;
    *) echo "Unsupported arch: $(uname -m)" && exit 1 ;;
esac

# Build download URL
FILE="${BIN_NAME}-linux-${ARCH}"
if [ -n "${MIRROR}" ]; then
    URL="${MIRROR}/https://github.com/${REPO}/releases/${VERSION}/download/${FILE}"
else
    URL="https://github.com/${REPO}/releases/${VERSION}/download/${FILE}"
fi

echo "Installing ${REPO} (${ARCH}) ..."

# Download and install
TMP=$(mktemp -d)
trap 'rm -rf $TMP' EXIT

curl -fsSL "${URL}" -o "${TMP}/${BIN_NAME}" || wget -qO "${TMP}/${BIN_NAME}" "${URL}"
chmod +x "${TMP}/${BIN_NAME}"
mkdir -p "${BIN_DIR}"
mv "${TMP}/${BIN_NAME}" "${BIN_DIR}/${BIN_NAME}"

echo "mihoro installed to ${BIN_DIR}/${BIN_NAME}"
if ! echo ":$PATH:" | grep -q ":${BIN_DIR}:"; then
    echo "NOTE: add ${BIN_DIR} to your PATH"
fi
