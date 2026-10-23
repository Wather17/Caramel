#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

case "${CARAMEL_ARCH:-$(uname -m)}" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Erro: arquitetura não suportada: $(uname -m). Use Linux amd64 ou arm64." >&2
        exit 1
        ;;
esac

SOURCE_PATH="${PROJECT_ROOT}/dist/caramel-linux-${ARCH}"
BUILD_DIR=""
cleanup() {
    if [[ -n "${BUILD_DIR}" ]]; then
        rm -rf "${BUILD_DIR}"
    fi
}
trap cleanup EXIT

if [[ ! -f "${SOURCE_PATH}" ]]; then
    if ! command -v go >/dev/null 2>&1; then
        echo "Erro: ${SOURCE_PATH} não existe e o Go não está instalado." >&2
        exit 1
    fi
    BUILD_DIR="$(mktemp -d)"
    SOURCE_PATH="${BUILD_DIR}/caramel"
    echo "🍬 Compilando Caramel CLI para Linux (${ARCH})..."
    (cd "${PROJECT_ROOT}" && GOOS=linux GOARCH="${ARCH}" go build -o "${SOURCE_PATH}" ./cmd/caramel)
else
    echo "🍬 Instalando Caramel CLI a partir de dist/caramel-linux-${ARCH}..."
fi

CARAMEL_SOURCE_PATH="${SOURCE_PATH}" bash "${SCRIPT_DIR}/install-linux.sh"
