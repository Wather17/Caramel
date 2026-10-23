#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
TEMP_ROOT="$(mktemp -d)"
GO_MODULE_CACHE="$(go env GOMODCACHE)"
GO_BUILD_CACHE="$(go env GOCACHE)"

cleanup() {
    chmod -R u+w -- "${TEMP_ROOT}" 2>/dev/null || true
    rm -rf -- "${TEMP_ROOT}"
}
trap cleanup EXIT

if ! command -v file >/dev/null 2>&1; then
    echo "Erro: o smoke test precisa do comando file." >&2
    exit 1
fi

for ARCH in amd64 arm64; do
    TEST_ROOT="${TEMP_ROOT}/${ARCH}"
    TEST_REPO="${TEST_ROOT}/repo"
    TEST_HOME="${TEST_ROOT}/home"
    mkdir -p "${TEST_REPO}" "${TEST_HOME}"

    # Use uma cópia sem dist/ para obrigar scripts/install.sh a exercitar o
    # fallback de compilação da arquitetura solicitada.
    tar \
        --exclude=./.git \
        --exclude=./dist \
        -C "${PROJECT_ROOT}" \
        -cf - . \
        | tar -C "${TEST_REPO}" -xf -

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

    description="$(file -b "${target}")"
    case "${ARCH}" in
        amd64)
            [[ "${description}" == *"x86-64"* ]] || {
                echo "Erro: binário amd64 inesperado: ${description}" >&2
                exit 1
            }
            "${target}" version >/dev/null
            ;;
        arm64)
            [[ "${description}" == *"ARM aarch64"* || "${description}" == *"ARM64"* ]] || {
                echo "Erro: binário arm64 inesperado: ${description}" >&2
                exit 1
            }
            ;;
    esac

    if [[ ! -L "${TEST_HOME}/.local/bin/mel" ]] || [[ "$(readlink "${TEST_HOME}/.local/bin/mel")" != "caramel" ]]; then
        echo "Erro: o alias mel não foi preservado no smoke test de ${ARCH}." >&2
        exit 1
    fi

    if [[ "${ARCH}" == amd64 ]]; then
        mkdir -p "${TEST_REPO}/dist"
        cp -- "${target}" "${TEST_REPO}/dist/caramel-linux-amd64"
        artifact_output="$(
            CARAMEL_ARCH=amd64 \
            HOME="${TEST_ROOT}/artifact-home" \
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
    fi
done

echo "Smoke tests do instalador Linux concluídos com sucesso."
