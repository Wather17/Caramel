# 🏛️ Arquitetura do Caramel CLI

O **Caramel CLI** é uma aplicação em Go projetada para fornecer um ecossistema de ferramentas de linha de comando voltadas para o desenvolvimento pedagógico e utilitários do dia a dia.

---

## 📁 Estrutura de Diretórios

```text
Caramel/
├── cmd/
│   └── caramel/             # Ponto de entrada (main.go)
├── docs/                    # Documentação técnica e Guias do sistema
│   ├── ARCHITECTURE.md      # Visão geral da arquitetura (este arquivo)
│   ├── GETTING_STARTED.md   # Guia de início rápido e compilação
│   └── CONTRIBUTING_COMMANDS.md # Como criar novos comandos e ferramentas
├── internal/                # Regras de negócio e código privado
│   ├── cli/                 # Comandos e subcomandos CLI (Cobra)
│   │   ├── root.go          # Comando raiz (`caramel`)
│   │   ├── workspace.go     # Entrada da área de trabalho visual
│   │   └── ...              # Comandos agrupados por fluxo
│   ├── config/              # Gerenciador de configurações e preferências
│   ├── ui/                  # Componentes TUI, tema, previews e workspace
│   ├── vault/               # Acervo global SQLite, objetos e migração legada
│   ├── workspace/           # Compatibilidade com projetos JSON do MVP
│   ├── workflow/            # Orquestração compartilhada pela CLI e pela TUI
│   └── tools/               # Módulos e motores das ferramentas pedagógicas
│       ├── ai/              # Clientes OpenRouter, triagem e geração
│       ├── cards/            # Geração de fichas A4
│       ├── docx/             # Leitura, extração e reconstrução de DOCX
│       ├── pdf/              # Geração de PDFs de impressão
│       └── pipeline/         # Pipelines compostos de DOCX
├── dist/                    # Binários gerados pela compilação (ignorado no git)
├── scripts/                 # Scripts automatizados
│   ├── build.sh             # Compilação cross-platform (Linux & Windows)
│   ├── install.sh           # Instalador automático para Linux
│   └── install.ps1          # Instalador automático para Windows (PowerShell)
├── go.mod                   # Gerenciador de módulos Go
├── go.sum
└── README.md
```

---

## 🧩 Componentes Principais

### 1. Entrypoint (`cmd/caramel/main.go`)
Contém apenas a chamada para `cli.Execute()`. A lógica de roteamento fica encapsulada no pacote `internal/cli`.

### 2. Pacote de CLI (`internal/cli/`)
Construído utilizando o framework **Cobra** (`github.com/spf13/cobra`).
- Cada novo subcomando deve residir nesta pasta ou em subpastas organizadas por domínio.
- Os comandos registram-se no `RootCmd` através da função `init()`.

### 3. Pacote de Ferramentas (`internal/tools/`)
Isola toda a lógica de negócio das ferramentas da CLI:
- Não deve conter código direto de CLI (como prints de flags ou parsing de argumentos de terminal).
- Retorna dados puros, estruturas Go ou erros formatados para o pacote `cli`.

### 4. Área de trabalho (`internal/vault/`, `internal/workflow/` e `internal/ui/`)

A área de trabalho visual é aberta com `caramel workspace` e funciona como uma inbox para
um acervo global. O usuário pensa em materiais, não em pastas: cada material é identificado
por hash, armazenado uma única vez em `objects/` e indexado em `vault.sqlite`. Coleções são
temporárias e apenas referenciam materiais; execuções registram entradas, saídas,
proveniência e derivações. O nome de um arquivo também pode declarar uma classificação
pedagógica opcional, separada do formato técnico: por exemplo, `at ciencias 01.png` vira
uma atividade com a tag `ciencias`.

O diretório do vault é escolhido por `CARAMEL_VAULT_DIR` ou pelo diretório de dados padrão
do sistema (`~/.local/share/caramel` no Linux). A TUI importa, pesquisa, seleciona e encadeia
geração, coloração e impressão sem criar pastas no diretório atual. Materiais arquivados
continuam preservados e podem ser incluídos explicitamente nas buscas.

O pacote `workspace` permanece como camada de compatibilidade para os manifestos JSON do
MVP. Na abertura da TUI, projetos antigos são migrados automaticamente para coleções e
materiais do vault, sem apagar os diretórios legados. Os comandos CLI existentes preservam
seus próprios contratos e destinos.

---

## 🔄 Fluxo de Execução

```mermaid
graph TD
    User([Usuário]) -->|Digita comando| Main[cmd/caramel/main.go]
    Main -->|Invoca| CLI[internal/cli/Execute]
    CLI -->|Parses flags & seleciona subcomando| Command[Subcomando ex: caramel image generate]
    Command -->|Chama regra de negócio| Tools[internal/tools/ai]
    Tools -->|Retorna dados/resultado| Command
    Command -->|Renderiza resposta no terminal| User

    Workspace[caramel workspace] --> UI[internal/ui]
    UI --> Vault[internal/vault]
    Vault --> SQLite[(vault.sqlite)]
    Vault --> Objects[(objects/<hash>.<ext>)]
    Vault --> Workflow[internal/workflow]
    Workflow --> Tools
```

---

## 📦 Compilação Cross-Platform (Linux & Windows)

O Caramel utiliza a compilação nativa do Go sem dependências C (`CGO_ENABLED=0`).

Os binários produzidos são:
- `caramel-linux-amd64` (Linux 64-bit)
- `caramel-linux-arm64` (Linux ARM 64-bit)
- `caramel-windows-amd64.exe` (Windows 64-bit)
