#!/usr/bin/env bash
set -e

INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="caramel"
ALIAS_NAME="mel"
ALIAS_PATH="${INSTALL_DIR}/${ALIAS_NAME}"

echo "🍬 Instalando Caramel CLI..."

# Ensure target directory exists
mkdir -p "${INSTALL_DIR}"

# Build binary for Linux if dist doesn't exist
if [ ! -f "dist/caramel-linux-amd64" ]; then
    echo " └─ Compilando executável..."
    go build -o "${INSTALL_DIR}/${BINARY_NAME}" ./cmd/caramel
else
    echo " └─ Copiando binário de dist/..."
    cp dist/caramel-linux-amd64 "${INSTALL_DIR}/${BINARY_NAME}"
fi

chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# Cria um alias seguro para o mesmo executável, sem substituir arquivos existentes.
if [ -L "${ALIAS_PATH}" ]; then
    if [ "$(readlink "${ALIAS_PATH}" 2>/dev/null || true)" = "${BINARY_NAME}" ]; then
        echo " └─ Alias '${ALIAS_NAME}' já está configurado."
    else
        echo " ⚠️  Alias '${ALIAS_NAME}' já existe e não é gerenciado pelo Caramel; mantido sem alteração." >&2
    fi
elif [ -e "${ALIAS_PATH}" ]; then
    echo " ⚠️  Arquivo '${ALIAS_PATH}' já existe e não é gerenciado pelo Caramel; mantido sem alteração." >&2
else
    ln -s "${BINARY_NAME}" "${ALIAS_PATH}"
    echo " └─ Alias '${ALIAS_NAME}' criado para '${BINARY_NAME}'."
fi

echo "✅ Caramel instalado com sucesso em: ${INSTALL_DIR}/${BINARY_NAME}"
echo "   Use 'caramel' ou 'mel' para executar o CLI."
echo ""
echo "Certifique-se de que '${INSTALL_DIR}' está no seu PATH no ~/.bashrc ou ~/.zshrc:"
echo '  export PATH="$HOME/.local/bin:$PATH"'
