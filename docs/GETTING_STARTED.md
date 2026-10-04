# 🚀 Guia de Início Rápido (Getting Started)

Este documento instrui como compilar, testar e instalar o **Caramel CLI** localmente em ambientes Linux e Windows.

---

## 📋 Pré-requisitos

- **Go** (versão 1.25 ou superior). Verifique com `go version`.
- **Git** (para captura dos hashes de commit na compilação).

Para usar a área de trabalho visual, o Caramel também precisa ser executado em um terminal
interativo compatível com cores ANSI.

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

### 2. Abrindo a área de trabalho visual

```bash
go run ./cmd/caramel workspace
```

A área de trabalho mantém um acervo global no diretório de dados do usuário. Os arquivos
importados são deduplicados por hash e as saídas ficam registradas no vault, sem criar
pastas de trabalho no diretório atual. Para escolher outro local, defina
`CARAMEL_VAULT_DIR` antes de iniciar.

Para configurar arquivos visíveis em `Documents/Caramel` e indexar um acervo existente sem
movê-lo:

```powershell
caramel vault init
caramel vault source add "C:\Users\55689\Documents\docs-mãe"
caramel vault sync
```

Use `CARAMEL_LIBRARY_DIR` para substituir a biblioteca visível. Essa variável é independente de
`CARAMEL_VAULT_DIR`, que continua reservado ao banco, cache e objetos internos.

No Windows, ao concluir `caramel vault init` em um terminal interativo, o Caramel pergunta se
deve ativar o autocomplete permanente do PowerShell. A confirmação configura os perfis do
PowerShell 7 e do Windows PowerShell 5.1 sem alterar suas outras personalizações.

Para ativar, verificar ou remover manualmente essa integração:

```powershell
caramel shell enable powershell
caramel shell status powershell
caramel shell disable powershell
```

O bloco gerenciado usa UTF-8 e insere caminhos com espaços entre aspas simples. Ao pressionar
`Tab` nos argumentos de DOCX, PDF, imagem, impressão ou rotina, o Caramel atualiza índices com
mais de 30 segundos e sugere apenas extensões aceitas pelo comando. O ranking considera texto,
descoberta/modificação/último uso e nome. Em caso de falha do vault, o PowerShell volta ao
autocomplete comum de arquivos sem imprimir diagnóstico na lista de sugestões.

---

### 3. Compilação Multiplataforma (Linux + Windows)
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

A instalação cria também o alias curto `mel`, apontando para o mesmo executável:

```bash
mel --help
mel version
```

### No Windows (PowerShell):
#### Opção A: Via Script Local
Execute o script de instalação em PowerShell:
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
```
O instalador disponibiliza `mel.cmd` junto com `caramel.exe`; ambos aceitam a mesma árvore
de comandos, flags, entrada e saída.
*(Nota: O script compila o binário automaticamente com `go build` caso a pasta `dist/` ainda não tenha sido gerada.)*

#### Opção B: Via Scoop (Recomendado)
Se você utiliza o **Scoop**, pode instalar o Caramel diretamente com o comando:
```powershell
scoop install https://raw.githubusercontent.com/Wather17/Caramel/main/bucket/caramel.json
```
O manifesto do Scoop cria os shims `caramel` e `mel` para o mesmo executável.
