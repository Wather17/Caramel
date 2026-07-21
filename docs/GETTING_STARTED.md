# 🚀 Guia de Início Rápido (Getting Started)

Este documento instrui como compilar, testar e instalar o **Caramel CLI** localmente em ambientes Linux e Windows.

---

## 📋 Pré-requisitos

- **Go** (versão 1.20 ou superior). Verifique com `go version`.
- **Git** (para captura dos hashes de commit na compilação).

---

## 🛠️ Compilando a Aplicação

### 1. Compilação Rápida para Desenvolvimento Local
Para rodar diretamente o código fonte sem compilar previamente:
```bash
go run ./cmd/caramel version
```

Ou compilar o binário para seu sistema atual:
```bash
go build -o caramel ./cmd/caramel
./caramel --help
```

---

### 2. Compilação Multiplataforma (Linux + Windows)
Para gerar executáveis para Linux e Windows simultaneamente em `dist/`:

#### No Linux / macOS:
```bash
chmod +x scripts/build.sh
./scripts/build.sh
```

Os seguintes arquivos serão gerados na pasta `dist/`:
- `dist/caramel-linux-amd64`
- `dist/caramel-linux-arm64`
- `dist/caramel-windows-amd64.exe`

---

## 💻 Instalando o Executável

### No Linux:
Execute o script de instalação para mover o binário para `~/.local/bin`:
```bash
chmod +x scripts/install.sh
./scripts/install.sh
```

### No Windows (PowerShell):
Execute o script de instalação em PowerShell:
```powershell
.\scripts\install.ps1
```
