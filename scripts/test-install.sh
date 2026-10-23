#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
TEMP_ROOT="$(mktemp -d)"
GO_MODULE_CACHE="$(go env GOMODCACHE)"
GO_BUILD_CACHE="$(go env GOCACHE)"
ARTIFACT_ROOT=""
SERVER_PID=""
PATH_MARKER_START="# >>> caramel path >>>"

if [[ -n "${CARAMEL_ARTIFACT_DIR:-}" ]]; then
    if [[ ! -d "${CARAMEL_ARTIFACT_DIR}" ]]; then
        echo "Erro: diretório de artefatos não encontrado: ${CARAMEL_ARTIFACT_DIR}" >&2
        exit 1
    fi
    ARTIFACT_ROOT="$(cd -- "${CARAMEL_ARTIFACT_DIR}" && pwd)"
fi

cleanup() {
    if [[ -n "${SERVER_PID}" ]]; then
        kill "${SERVER_PID}" 2>/dev/null || true
        wait "${SERVER_PID}" 2>/dev/null || true
    fi
    chmod -R u+w -- "${TEMP_ROOT}" 2>/dev/null || true
    rm -rf -- "${TEMP_ROOT}"
}
trap cleanup EXIT

if ! command -v file >/dev/null 2>&1; then
    echo "Erro: o smoke test precisa do comando file." >&2
    exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
    echo "Erro: o smoke test precisa do Python 3 para servir as fixtures locais." >&2
    exit 1
fi

assert_architecture() {
    local arch="$1"
    local path="$2"
    local description
    description="$(file -b "${path}")"
    case "${arch}" in
        amd64)
            [[ "${description}" == *"x86-64"* ]] || {
                echo "Erro: binário amd64 inesperado: ${description}" >&2
                exit 1
            }
            ;;
        arm64)
            [[ "${description}" == *"ARM aarch64"* || "${description}" == *"ARM64"* ]] || {
                echo "Erro: binário arm64 inesperado: ${description}" >&2
                exit 1
            }
            ;;
        windows-amd64)
            [[ "${description}" == *"PE32+"* && "${description}" == *"x86-64"* ]] || {
                echo "Erro: binário Windows amd64 inesperado: ${description}" >&2
                exit 1
            }
            ;;
    esac
}

verify_release_artifacts() {
    local expected
    for expected in caramel-linux-amd64 caramel-linux-arm64 caramel-windows-amd64.exe SHA256SUMS; do
        if [[ ! -f "${ARTIFACT_ROOT}/${expected}" ]]; then
            echo "Erro: artefato ausente: ${ARTIFACT_ROOT}/${expected}" >&2
            exit 1
        fi
    done

    assert_architecture amd64 "${ARTIFACT_ROOT}/caramel-linux-amd64"
    assert_architecture arm64 "${ARTIFACT_ROOT}/caramel-linux-arm64"
    assert_architecture windows-amd64 "${ARTIFACT_ROOT}/caramel-windows-amd64.exe"
    if ! (cd "${ARTIFACT_ROOT}" && sha256sum -c SHA256SUMS --ignore-missing); then
        echo "Erro: SHA256SUMS não confere com os artefatos publicados." >&2
        exit 1
    fi
}

copy_project_without_dist() {
    local destination="$1"
    mkdir -p "${destination}"
    tar \
        --exclude=./.git \
        --exclude=./dist \
        -C "${PROJECT_ROOT}" \
        -cf - . \
        | tar -C "${destination}" -xf -
}

assert_alias() {
    local home="$1"
    local arch="$2"
    if [[ ! -L "${home}/.local/bin/mel" ]] || [[ "$(readlink "${home}/.local/bin/mel")" != "caramel" ]]; then
        echo "Erro: o alias mel não foi preservado no smoke test de ${arch}." >&2
        exit 1
    fi
}

assert_profile_is_idempotent() {
    local home="$1"
    local profile="${home}/.bashrc"
    if ! grep -Fq "conteúdo externo preservado" "${profile}"; then
        echo "Erro: o instalador removeu conteúdo externo do perfil." >&2
        exit 1
    fi
    if [[ "$(grep -Fc "${PATH_MARKER_START}" "${profile}")" -ne 1 ]]; then
        echo "Erro: o bloco PATH do instalador não é idempotente." >&2
        exit 1
    fi
}

build_versioned_binary() {
    local version="$1"
    local output="$2"
    local ldflags="-X caramel/internal/cli.Version=${version}"
    (
        cd "${PROJECT_ROOT}"
        GOOS=linux GOARCH=amd64 \
            GOMODCACHE="${GO_MODULE_CACHE}" \
            GOCACHE="${GO_BUILD_CACHE}" \
            go build -ldflags "${ldflags}" -o "${output}" ./cmd/caramel
    )
}

start_fixture_server() {
    local fixture_root="$1"
    local port_file="${fixture_root}/.port"

    python3 - "${port_file}" "${fixture_root}" <<'PY' &
import functools
import http.server
import pathlib
import sys

port_file = pathlib.Path(sys.argv[1])
root = sys.argv[2]

class QuietHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

server = http.server.ThreadingHTTPServer(
    ("127.0.0.1", 0), functools.partial(QuietHandler, directory=root)
)
port_file.write_text(str(server.server_port), encoding="ascii")
server.serve_forever()
PY
    SERVER_PID=$!

    for _ in $(seq 1 50); do
        if [[ -s "${port_file}" ]]; then
            FIXTURE_BASE="http://127.0.0.1:$(<"${port_file}")"
            return
        fi
        sleep 0.1
    done
    echo "Erro: servidor de fixture local não iniciou." >&2
    exit 1
}

refresh_fixture_checksum() {
    local fixture_root="$1"
    (cd "${fixture_root}" && sha256sum caramel-linux-amd64 > SHA256SUMS)
}

run_release_install() {
    local home="$1"
    CARAMEL_ARCH=amd64 \
        CARAMEL_VERSION=fixture \
        CARAMEL_RELEASE_BASE="${FIXTURE_BASE}" \
        HOME="${home}" \
        SHELL=/bin/bash \
        bash "${PROJECT_ROOT}/scripts/install-linux.sh" 2>&1
}

if [[ -n "${ARTIFACT_ROOT}" ]]; then
    echo "Validando artefatos em ${ARTIFACT_ROOT}..."
    verify_release_artifacts
fi

for ARCH in amd64 arm64; do
    TEST_ROOT="${TEMP_ROOT}/${ARCH}"
    TEST_REPO="${TEST_ROOT}/repo"
    TEST_HOME="${TEST_ROOT}/home"
    mkdir -p "${TEST_REPO}" "${TEST_HOME}"
    printf '# conteúdo externo preservado\n' > "${TEST_HOME}/.bashrc"

    # Use uma cópia sem dist/ para obrigar scripts/install.sh a exercitar o
    # fallback de compilação da arquitetura solicitada.
    copy_project_without_dist "${TEST_REPO}"

    echo "Testando fallback do instalador para Linux ${ARCH}..."
    output="$(
        CARAMEL_ARCH="${ARCH}" \
        HOME="${TEST_HOME}" \
        SHELL=/bin/bash \
        GOMODCACHE="${GO_MODULE_CACHE}" \
        GOCACHE="${GO_BUILD_CACHE}" \
        bash "${TEST_REPO}/scripts/install.sh" 2>&1
    )"
    printf '%s\n' "${output}"

    target="${TEST_HOME}/.local/bin/caramel"
    if [[ ! -x "${target}" ]]; then
        echo "Erro: o instalador não criou um executável para ${ARCH}." >&2
        exit 1
    fi
    assert_architecture "${ARCH}" "${target}"
    if [[ "${ARCH}" == amd64 ]]; then
        "${target}" version >/dev/null
    fi
    assert_alias "${TEST_HOME}" "${ARCH}"
    assert_profile_is_idempotent "${TEST_HOME}"

    artifact_source=""
    if [[ -n "${ARTIFACT_ROOT}" ]]; then
        artifact_source="${ARTIFACT_ROOT}/caramel-linux-${ARCH}"
        mkdir -p "${TEST_REPO}/dist"
        cp -- "${artifact_source}" "${TEST_REPO}/dist/caramel-linux-${ARCH}"
    elif [[ "${ARCH}" == amd64 ]]; then
        mkdir -p "${TEST_REPO}/dist"
        artifact_source="${TEST_REPO}/dist/caramel-linux-amd64"
        cp -- "${target}" "${artifact_source}"
    fi

    if [[ -n "${artifact_source}" ]]; then
        artifact_home="${TEST_ROOT}/artifact-home"
        mkdir -p "${artifact_home}"
        printf '# conteúdo externo preservado\n' > "${artifact_home}/.bashrc"
        artifact_output="$(
            CARAMEL_ARCH="${ARCH}" \
            HOME="${artifact_home}" \
            SHELL=/bin/bash \
            GOMODCACHE="${GO_MODULE_CACHE}" \
            GOCACHE="${GO_BUILD_CACHE}" \
            bash "${TEST_REPO}/scripts/install.sh" 2>&1
        )"
        printf '%s\n' "${artifact_output}"
        if [[ "${artifact_output}" == *"Compilando Caramel CLI"* ]]; then
            echo "Erro: um artefato existente foi recompilado inesperadamente." >&2
            exit 1
        fi
        artifact_target="${artifact_home}/.local/bin/caramel"
        assert_architecture "${ARCH}" "${artifact_target}"
        if [[ "${ARCH}" == amd64 ]]; then
            "${artifact_target}" version >/dev/null
        fi
        assert_alias "${artifact_home}" "${ARCH}"
        assert_profile_is_idempotent "${artifact_home}"
    fi
done

UPGRADE_ROOT="${TEMP_ROOT}/download-update"
FIXTURE_ROOT="${UPGRADE_ROOT}/fixture"
UPGRADE_HOME="${UPGRADE_ROOT}/home"
mkdir -p "${FIXTURE_ROOT}" "${UPGRADE_HOME}"
printf '# conteúdo externo preservado\n' > "${UPGRADE_HOME}/.bashrc"
build_versioned_binary "smoke-old" "${UPGRADE_ROOT}/caramel-old"
build_versioned_binary "smoke-new" "${UPGRADE_ROOT}/caramel-new"
cp -- "${UPGRADE_ROOT}/caramel-old" "${FIXTURE_ROOT}/caramel-linux-amd64"
refresh_fixture_checksum "${FIXTURE_ROOT}"
start_fixture_server "${FIXTURE_ROOT}"

echo "Testando instalação por release, atualização e PATH idempotente..."
download_output="$(run_release_install "${UPGRADE_HOME}")"
printf '%s\n' "${download_output}"
if [[ "$("${UPGRADE_HOME}/.local/bin/caramel" version)" != *"smoke-old"* ]]; then
    echo "Erro: a primeira versão da fixture não foi instalada." >&2
    exit 1
fi
assert_alias "${UPGRADE_HOME}" "release"
assert_profile_is_idempotent "${UPGRADE_HOME}"

cp -- "${UPGRADE_ROOT}/caramel-new" "${FIXTURE_ROOT}/caramel-linux-amd64"
refresh_fixture_checksum "${FIXTURE_ROOT}"
download_output="$(run_release_install "${UPGRADE_HOME}")"
printf '%s\n' "${download_output}"
if [[ "$("${UPGRADE_HOME}/.local/bin/caramel" version)" != *"smoke-new"* ]]; then
    echo "Erro: a atualização da fixture não substituiu a versão instalada." >&2
    exit 1
fi
assert_alias "${UPGRADE_HOME}" "release"
assert_profile_is_idempotent "${UPGRADE_HOME}"

installed_hash="$(sha256sum "${UPGRADE_HOME}/.local/bin/caramel" | awk '{print $1}')"
printf '%s\n' "0000000000000000000000000000000000000000000000000000000000000000  caramel-linux-amd64" > "${FIXTURE_ROOT}/SHA256SUMS"
if checksum_output="$(run_release_install "${UPGRADE_HOME}" 2>&1)"; then
    echo "Erro: checksum inválido foi aceito." >&2
    exit 1
else
    printf '%s\n' "${checksum_output}"
fi
if [[ "$(sha256sum "${UPGRADE_HOME}/.local/bin/caramel" | awk '{print $1}')" != "${installed_hash}" ]]; then
    echo "Erro: checksum inválido alterou a instalação existente." >&2
    exit 1
fi

refresh_fixture_checksum "${FIXTURE_ROOT}"
mv -- "${FIXTURE_ROOT}/caramel-linux-amd64" "${FIXTURE_ROOT}/caramel-linux-amd64.missing"
if missing_output="$(run_release_install "${UPGRADE_HOME}" 2>&1)"; then
    echo "Erro: asset ausente foi aceito." >&2
    exit 1
else
    printf '%s\n' "${missing_output}"
fi
mv -- "${FIXTURE_ROOT}/caramel-linux-amd64.missing" "${FIXTURE_ROOT}/caramel-linux-amd64"

if unsupported_output="$(
    CARAMEL_ARCH=ppc64 \
        CARAMEL_RELEASE_BASE="${FIXTURE_BASE}" \
        HOME="${UPGRADE_HOME}" \
        SHELL=/bin/bash \
        bash "${PROJECT_ROOT}/scripts/install-linux.sh" 2>&1
)"; then
    echo "Erro: arquitetura incompatível foi aceita." >&2
    exit 1
else
    printf '%s\n' "${unsupported_output}"
fi

echo "Smoke tests do instalador Linux concluídos com sucesso."
