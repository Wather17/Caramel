#!/usr/bin/env bash
set -euo pipefail

# Instalador por usuário para Linux. Pode ser usado diretamente pela release:
#   curl -fsSL https://raw.githubusercontent.com/Wather17/Caramel/main/scripts/install-linux.sh | bash
#
# CARAMEL_SOURCE_PATH permite que scripts/install.sh reutilize toda a lógica de
# instalação quando estiver instalando um binário compilado localmente.

REPOSITORY="${CARAMEL_REPOSITORY:-Wather17/Caramel}"
INSTALL_DIR="${CARAMEL_INSTALL_DIR:-${HOME}/.local/bin}"
SOURCE_PATH="${CARAMEL_SOURCE_PATH:-}"
REQUESTED_VERSION="${CARAMEL_VERSION:-latest}"

if [[ "$(uname -s)" != "Linux" ]]; then
    echo "Erro: este instalador é destinado ao Linux." >&2
    exit 1
fi

if [[ -z "${HOME:-}" ]]; then
    echo "Erro: a variável HOME não está definida." >&2
    exit 1
fi

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

ASSET_NAME="caramel-linux-${ARCH}"
TEMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TEMP_DIR}"
}
trap cleanup EXIT

SOURCE_TO_INSTALL="${SOURCE_PATH}"
if [[ -n "${SOURCE_TO_INSTALL}" ]]; then
    if [[ ! -f "${SOURCE_TO_INSTALL}" ]]; then
        echo "Erro: binário de origem não encontrado: ${SOURCE_TO_INSTALL}" >&2
        exit 1
    fi
    echo "🍬 Instalando Caramel CLI a partir do binário local..."
else
    if ! command -v curl >/dev/null 2>&1; then
        echo "Erro: este instalador precisa do curl." >&2
        exit 1
    fi

    if [[ "${REQUESTED_VERSION}" == "latest" ]]; then
        RELEASE_BASE="https://github.com/${REPOSITORY}/releases/latest/download"
    else
        VERSION_WITHOUT_V="${REQUESTED_VERSION#v}"
        RELEASE_BASE="https://github.com/${REPOSITORY}/releases/download/v${VERSION_WITHOUT_V}"
    fi

    SOURCE_TO_INSTALL="${TEMP_DIR}/${ASSET_NAME}"
    CHECKSUMS_PATH="${TEMP_DIR}/SHA256SUMS"
    echo "🍬 Baixando Caramel CLI (${ARCH})..."
    curl -fL --retry 3 --retry-delay 1 "${RELEASE_BASE}/${ASSET_NAME}" -o "${SOURCE_TO_INSTALL}"

    # Releases antigas podem não ter checksums. A partir das próximas releases,
    # a presença do arquivo garante a validação do binário antes da instalação.
    if curl -fL --retry 3 --retry-delay 1 "${RELEASE_BASE}/SHA256SUMS" -o "${CHECKSUMS_PATH}" 2>/dev/null; then
        EXPECTED_HASH="$(awk -v asset="${ASSET_NAME}" '$2 == asset || $2 == "*" asset { print $1; exit }' "${CHECKSUMS_PATH}")"
        if [[ -z "${EXPECTED_HASH}" ]]; then
            echo "Erro: o checksum de ${ASSET_NAME} não foi encontrado em SHA256SUMS." >&2
            exit 1
        fi
        ACTUAL_HASH="$(sha256sum "${SOURCE_TO_INSTALL}" | awk '{print $1}')"
        if [[ "${EXPECTED_HASH}" != "${ACTUAL_HASH}" ]]; then
            echo "Erro: o checksum do binário baixado não confere." >&2
            exit 1
        fi
        echo " └─ Checksum verificado."
    else
        echo " ⚠️  A release ainda não possui SHA256SUMS; seguindo sem essa verificação." >&2
    fi
fi

mkdir -p "${INSTALL_DIR}"
TARGET_PATH="${INSTALL_DIR}/caramel"
TEMP_TARGET="${INSTALL_DIR}/.caramel-install-${BASHPID}"

# Publica a cópia de forma atômica dentro do mesmo diretório.
install -m 0755 "${SOURCE_TO_INSTALL}" "${TEMP_TARGET}"
mv -f "${TEMP_TARGET}" "${TARGET_PATH}"

ALIAS_PATH="${INSTALL_DIR}/mel"
if [[ -L "${ALIAS_PATH}" ]]; then
    ALIAS_TARGET="$(readlink "${ALIAS_PATH}" 2>/dev/null || true)"
    if [[ "${ALIAS_TARGET}" == "caramel" || "${ALIAS_TARGET}" == "${TARGET_PATH}" ]]; then
        :
    else
        echo " ⚠️  O alias '${ALIAS_PATH}' já existe e não é gerenciado pelo Caramel; mantido sem alteração." >&2
    fi
elif [[ -e "${ALIAS_PATH}" ]]; then
    echo " ⚠️  O caminho '${ALIAS_PATH}' já existe e não é gerenciado pelo Caramel; mantido sem alteração." >&2
else
    ln -s "caramel" "${ALIAS_PATH}"
fi

PATH_MARKER_START="# >>> caramel path >>>"
PATH_MARKER_END="# <<< caramel path <<<"
if [[ ":${PATH:-}:" != *":${INSTALL_DIR}:"* ]]; then
    SHELL_NAME="${SHELL:-}"
    SHELL_NAME="${SHELL_NAME##*/}"
    case "${SHELL_NAME}" in
        zsh)
            PROFILE_PATH="${ZDOTDIR:-${HOME}}/.zshrc"
            ;;
        bash)
            PROFILE_PATH="${HOME}/.bashrc"
            ;;
        *)
            PROFILE_PATH="${HOME}/.profile"
            ;;
    esac

    if [[ ! -f "${PROFILE_PATH}" ]] || ! grep -Fq "${PATH_MARKER_START}" "${PROFILE_PATH}"; then
        mkdir -p "$(dirname "${PROFILE_PATH}")"
        {
            [[ ! -s "${PROFILE_PATH}" ]] || printf '\n'
            printf '%s\n' "${PATH_MARKER_START}"
            printf 'export PATH="%s:$PATH"\n' "${INSTALL_DIR}"
            printf '%s\n' "${PATH_MARKER_END}"
        } >> "${PROFILE_PATH}"
        PROFILE_UPDATED="${PROFILE_PATH}"
    else
        PROFILE_UPDATED="${PROFILE_PATH}"
    fi
else
    PROFILE_UPDATED=""
fi

echo "✅ Caramel instalado com sucesso em: ${TARGET_PATH}"
echo "   Use 'caramel' ou 'mel' para executar o CLI."
if [[ -n "${PROFILE_UPDATED}" ]]; then
    echo "   Abra um novo terminal ou execute: source ${PROFILE_UPDATED}"
fi
