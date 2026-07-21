#!/usr/bin/env bash
set -e

INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="caramel"

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

echo "✅ Caramel instalado com sucesso em: ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "Certifique-se de que '${INSTALL_DIR}' está no seu PATH no ~/.bashrc ou ~/.zshrc:"
echo '  export PATH="$HOME/.local/bin:$PATH"'
