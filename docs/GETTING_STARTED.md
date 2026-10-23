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
Para instalar a release mais recente sem Go, execute:

```bash
curl -fsSL https://raw.githubusercontent.com/Wather17/Caramel/main/scripts/install-linux.sh | bash
```

O instalador detecta `amd64` ou `arm64`, instala o binário em `~/.local/bin`, cria o alias `mel` e
adiciona esse diretório ao perfil do shell quando necessário. Abra um novo terminal ou carregue o perfil:

```bash
source ~/.bashrc  # ou source ~/.zshrc
```

Para instalar a partir do checkout local, use o script abaixo. Ele usa o binário correspondente em `dist/`
ou compila o executável para Linux na arquitetura atual. Para selecionar explicitamente uma arquitetura
suportada, defina `CARAMEL_ARCH=amd64` ou `CARAMEL_ARCH=arm64`:

```bash
bash scripts/install.sh
```

As duas formas criam também o alias curto `mel`, apontando para o mesmo executável:

```bash
mel --help
mel version
```

Se você usar `caramel install` com um binário já baixado, o Linux persiste `~/.local/bin` no perfil do
shell reconhecido (`.bashrc`, `${ZDOTDIR:-$HOME}/.zshrc` ou `config.fish`). A saída informa o arquivo
alterado e o comando `source` correspondente; shells não reconhecidos recebem apenas uma orientação
manual, sem modificar um perfil escolhido arbitrariamente.

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
