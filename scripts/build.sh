#!/usr/bin/env bash
set -e

VERSION=${1:-"0.1.0-dev"}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-X caramel/internal/cli.Version=${VERSION} -X caramel/internal/cli.Commit=${COMMIT} -X caramel/internal/cli.Date=${DATE}"

OUTPUT_DIR="dist"
mkdir -p "${OUTPUT_DIR}"

echo "🍬 Compilando Caramel CLI (v${VERSION})..."

# Linux amd64
echo " └─ Compilando para Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "${OUTPUT_DIR}/caramel-linux-amd64" ./cmd/caramel

# Linux arm64
echo " └─ Compilando para Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o "${OUTPUT_DIR}/caramel-linux-arm64" ./cmd/caramel

# Windows amd64
echo " └─ Compilando para Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "${OUTPUT_DIR}/caramel-windows-amd64.exe" ./cmd/caramel

echo "✅ Compilação concluída com sucesso! Os binários estão disponíveis em ./${OUTPUT_DIR}/"
