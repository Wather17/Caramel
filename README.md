<div align="center">

# 🍬 Caramel CLI

**Do material à atividade pronta para imprimir.**

Ferramentas para trabalhar com DOCX, PDF, imagens e rotinas pedagógicas — no terminal ou em uma área de trabalho visual.

[![CI](https://github.com/Wather17/Caramel/actions/workflows/ci.yml/badge.svg)](https://github.com/Wather17/Caramel/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Wather17/Caramel?color=f5a97f)](https://github.com/Wather17/Caramel/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Licença MIT](https://img.shields.io/badge/Licen%C3%A7a-MIT-a6da95)](LICENSE)

[Instalação](#instalação) · [Primeiros passos](#primeiros-passos) · [Exemplos](#exemplos-de-uso) · [Documentação](#documentação)

</div>

---

## O que dá para fazer?

O Caramel é uma CLI escrita em Go para preparar materiais pedagógicos e automatizar tarefas de produção. Você pode executar uma operação diretamente ou abrir a **workspace** para organizar seu acervo e encadear os fluxos.

| Fluxo | Recursos |
| --- | --- |
| 📄 **Documentos Word** | Listar e extrair imagens, juntar documentos e dividir DOCX por quebras explícitas de página ou seção. |
| 📑 **PDFs** | Criar PDFs a partir de imagens, juntar e dividir arquivos, exportar páginas como imagens e extrair imagens embutidas. |
| 🖨️ **Impressão** | Montar duas atividades por folha A4 e gerar fichas com legendas em PDF ou HTML. |
| 🎨 **Imagens com IA** | Gerar coleções de ilustrações, colorir imagens e DOCX e reutilizar imagens pela biblioteca local. |
| 📅 **Rotinas de aula** | Consolidar rotinas semanais de arquivos DOCX e classificar Campos de Experiência da BNCC com IA. |
| 🗂️ **Workspace visual** | Importar, pesquisar e selecionar materiais em um acervo global, com deduplicação e registro de origem e operações. |

As ferramentas locais de DOCX, PDF e impressão funcionam sem chave de API. Os fluxos de IA usam o **OpenRouter** e precisam de uma chave e acesso aos modelos escolhidos.

## Instalação

Os binários publicados nas [releases](https://github.com/Wather17/Caramel/releases/latest) estão prontos para uso, sem precisar instalar Go.

| Sistema | Arquitetura | Arquivo da release |
| --- | --- | --- |
| Linux / WSL | x86_64 / AMD64 | `caramel-linux-amd64` |
| Linux / WSL | ARM64 / aarch64 | `caramel-linux-arm64` |
| Windows | x86_64 / AMD64 | `caramel-windows-amd64.exe` |

No WSL, use o procedimento de **Linux** dentro da distribuição. O fluxo de releases publica binários para os sistemas e arquiteturas da tabela.

### Linux / WSL: baixar a release

Você precisa de `curl`. Confira a arquitetura com `uname -m`: `x86_64` corresponde a `amd64`; `aarch64` ou `arm64` corresponde a `arm64`.

Em uma pasta de downloads, escolha a arquitetura e execute:

```bash
# Use arm64 se o seu sistema for ARM64.
caramel_arch=amd64

curl -fL "https://github.com/Wather17/Caramel/releases/latest/download/caramel-linux-${caramel_arch}" \
  -o "caramel-linux-${caramel_arch}"
chmod +x "caramel-linux-${caramel_arch}"
"./caramel-linux-${caramel_arch}" install
```

O comando copia o executável para **`~/.local/bin/caramel`**. Para disponibilizá-lo no terminal atual:

```bash
export PATH="$HOME/.local/bin:$PATH"
caramel version
caramel --help
```

Para manter o PATH nas próximas sessões, adicione a linha `export PATH="$HOME/.local/bin:$PATH"` ao arquivo de configuração do seu shell, como `~/.bashrc` ou `~/.zshrc`, e reabra o terminal.

### Windows: via Scoop

Se o [Scoop](https://scoop.sh/) já estiver instalado, execute no PowerShell:

```powershell
scoop install https://raw.githubusercontent.com/Wather17/Caramel/main/bucket/caramel.json
caramel version
mel --help
```

O manifesto disponibiliza os comandos **`caramel`** e **`mel`** para o mesmo executável.

### Windows: baixar o executável

Como alternativa ao Scoop, baixe e instale a release pelo PowerShell em uma pasta de downloads:

```powershell
Invoke-WebRequest `
  -Uri "https://github.com/Wather17/Caramel/releases/latest/download/caramel-windows-amd64.exe" `
  -OutFile ".\caramel-windows-amd64.exe"

.\caramel-windows-amd64.exe install
```

O instalador copia o executável para **`%USERPROFILE%\.caramel\bin\caramel.exe`** e adiciona a pasta ao PATH do usuário. Reabra o PowerShell e confira:

```powershell
caramel version
caramel --help
```

### Instalar a partir do código fonte

Para compilar localmente, instale **Git** e **Go 1.25 ou superior**. Clone o repositório e entre na pasta:

```bash
git clone https://github.com/Wather17/Caramel.git
cd Caramel
```

**Linux AMD64**, a partir da raiz do repositório:

```bash
bash scripts/install.sh
export PATH="$HOME/.local/bin:$PATH"
caramel --help
mel --help
```

O script usa `dist/caramel-linux-amd64` quando esse arquivo existe; caso contrário, compila para o sistema atual. Em **Linux ARM64**, compile explicitamente o binário para sua máquina e instale a cópia recém compilada:

```bash
go build -o caramel-local ./cmd/caramel
./caramel-local install
export PATH="$HOME/.local/bin:$PATH"
caramel --help
```

**Windows**, a partir da raiz do repositório no PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
caramel --help
mel --help
```

O script usa `dist\caramel-windows-amd64.exe` quando disponível e compila com Go quando o arquivo não existe. A instalação fica em `%USERPROFILE%\.caramel\bin`.

Os scripts locais criam o atalho `mel` quando o caminho está disponível e preservam atalhos existentes que não sejam gerenciados pelo Caramel. O comando `install` embutido no executável instala o nome `caramel`; `mel` é disponibilizado pelo Scoop e pelos scripts locais.

### Atualizar

Com Scoop:

```powershell
scoop update
scoop update caramel
caramel version
```

Para instalação por download, repita o procedimento da sua plataforma com a release mais recente, executando `install` a partir do **arquivo recém baixado na pasta de downloads**. Para uma compilação própria, atualize o checkout e recompile; os scripts locais dão preferência ao binário existente em `dist/`, então esse arquivo precisa estar atualizado se for usado.

## Primeiros passos

Descubra os comandos e consulte exemplos diretamente no terminal:

```bash
caramel guide
caramel guide pdf
caramel docx images extract --help
```

### Configurar os recursos de IA

Use o assistente para cadastrar sua chave do OpenRouter e selecione os modelos de imagem, texto e triagem:

```bash
caramel config setup
caramel config models select
caramel config show
```

Também é possível fornecer a chave pela variável `OPENROUTER_API_KEY`. As chamadas de IA usam serviços externos e podem consumir créditos da sua conta conforme os modelos escolhidos.

### Abrir a workspace

```bash
caramel workspace
```

Execute em um terminal interativo com suporte a cores ANSI. Na inbox, use `/` para buscar, `i` para importar e `g`, `c`, `f` e `p` para geração, coloração, fichas e montagem de duas atividades por folha.

O acervo é armazenado por padrão em `~/.local/share/caramel` no Linux e em `%LOCALAPPDATA%\caramel` no Windows. No Linux, `XDG_DATA_HOME` é respeitado; em qualquer plataforma, **`CARAMEL_VAULT_DIR`** permite escolher outro local. Os materiais são deduplicados por hash e as operações mantêm sua proveniência.

### Configurar a biblioteca visível

O banco e o cache continuam no diretório interno acima, enquanto os arquivos do usuário podem
ficar em uma biblioteca visível e configurável. No primeiro uso:

```powershell
caramel vault init "C:\Users\55689\Documents\Caramel"
caramel vault source add "C:\Users\55689\Documents\docs-mãe"
caramel vault sync
caramel vault status
```

`vault init` cria `materiais` e `resultados`. Fontes adicionadas são apenas indexadas: o Caramel
não move, renomeia nem apaga seus arquivos. A sincronização normal compara metadados e recalcula
hash somente para arquivos novos ou alterados; `caramel vault sync --full` força uma verificação
completa para diagnóstico.

No Windows, `caramel vault init` pergunta ao fim de uma inicialização interativa se deve ativar
o autocomplete permanente. A integração preserva as demais personalizações dos perfis PowerShell
7 e Windows PowerShell 5.1 e pode ser administrada manualmente:

```powershell
caramel shell enable powershell
caramel shell status powershell
caramel shell disable powershell
```

Depois disso, `Tab` sugere primeiro os arquivos compatíveis do índice, considerando o texto
digitado e a atividade recente. Caminhos com espaços e acentos são inseridos entre aspas simples
e em UTF-8. Se o índice estiver indisponível, o completador normal do sistema de arquivos
continua funcionando.

Com a biblioteca configurada, os comandos que produzem arquivos usam por padrão um diário de
resultados. Por exemplo, uma execução em 3 de outubro de 2026 fica sob:

```text
Caramel/
└── resultados/
    └── 2026-10-03/
        ├── docx/
        ├── imagens/
        ├── impressao/
        ├── pdf/
        └── rotinas/
```

Arquivos únicos ficam diretamente na categoria; conjuntos, como páginas renderizadas ou imagens
extraídas, permanecem juntos em uma subpasta. Repetições recebem `-2`, `-3` e assim por diante,
sem sobrescrever resultados anteriores. Uma flag `--output` ou `--output-dir` continua vencendo
e usa exatamente o destino informado. Sem biblioteca configurada, os destinos antigos ao lado da
entrada são preservados.

Para voltar imediatamente ao que acabou de ser produzido:

```powershell
caramel open last       # abre o arquivo no app padrão ou a pasta do pacote
caramel reveal last     # seleciona o arquivo no Explorer ou abre a pasta do pacote
caramel pdf create .\imagens --open
```

`--open` está disponível nos comandos que produzem arquivos e só inicia o aplicativo depois da
publicação e do registro da execução. A flag é incompatível com `--json` e `--quiet`; sem ela,
nenhum aplicativo gráfico é iniciado.

## Exemplos de uso

### Extrair imagens de um DOCX

```bash
caramel docx images list atividade.docx
caramel docx images extract atividade.docx --output ./imagens
```

### Preparar materiais para impressão

```bash
# Duas atividades por folha A4.
caramel print 2up ./imagens --output atividades.pdf

# Fichas com legendas em PDF ou HTML.
caramel print cards ./imagens --output fichas.pdf
caramel print cards ./imagens --html
```

### Organizar PDFs

```bash
caramel pdf create ./imagens --output apostila.pdf
caramel pdf merge capa.pdf apostila.pdf --output material.pdf
caramel pdf split material.pdf --ranges 1-3,4-6 --output-dir ./partes
```

Para exportar a aparência completa das páginas ou recuperar as imagens embutidas:

```bash
caramel pdf pages render apostila.pdf --format png --dpi 150 --output-dir ./paginas
caramel pdf images extract apostila.pdf --output-dir ./imagens-pdf
```

### Gerar e colorir imagens com IA

Após configurar o OpenRouter:

```bash
caramel image generate --items "maçã, banana, uva" --style clipart --output ./frutas
caramel print cards ./frutas --output fichas-frutas.pdf
caramel image colorize atividade.docx
```

Use `--reuse-cache` na geração para consultar e salvar imagens na biblioteca local. `--refresh-cache` gera novas imagens e atualiza a versão ativa.

### Consolidar rotinas de aula com IA

```bash
caramel routine consolidate ./rotinas --output ./relatorios
```

Os comandos que oferecem saída estruturada aceitam `--json`; use `--verbose` para detalhes de diagnóstico. Consulte o [contrato de saída](docs/OUTPUT_UX.md) para os modos disponíveis e suas compatibilidades.

## Documentação

| Guia | Conteúdo |
| --- | --- |
| [Referência de comandos](docs/COMMANDS.md) | Fluxos, exemplos, flags e aliases compatíveis. |
| [Início rápido](docs/GETTING_STARTED.md) | Compilação e execução para desenvolvimento. |
| [Arquitetura](docs/ARCHITECTURE.md) | Pacotes, dados e organização do projeto. |
| [Integrações de API](docs/API_INTEGRATIONS.md) | OpenRouter, concorrência, retries e limites. |
| [Saída da CLI](docs/OUTPUT_UX.md) | Mensagens, diagnóstico, JSON e modo silencioso. |
| [Design system](docs/DESIGN_SYSTEM.md) | Cores e padrões da interface visual. |
| [Criação de comandos](docs/CONTRIBUTING_COMMANDS.md) | Convenções para novas ferramentas. |
| [Releases](docs/RELEASES.md) | Versionamento e publicação. |

## Desenvolvimento e contribuição

Na raiz do checkout, você pode executar diretamente o código fonte:

```bash
go run ./cmd/caramel --help
go run ./cmd/caramel workspace
```

Para validar e compilar:

```bash
go test ./...
go vet ./...
go build -o caramel-local ./cmd/caramel
```

O script `bash scripts/build.sh` gera os três binários de distribuição em `dist/`. Ele também aceita a versão como primeiro argumento, **sem o prefixo `v`**, para preencher os metadados da compilação.

Encontrou um problema ou tem uma ideia? Abra uma [issue](https://github.com/Wather17/Caramel/issues) com o comando usado, o resultado esperado e o comportamento observado. Para adicionar ferramentas, consulte o [guia de contribuição](docs/CONTRIBUTING_COMMANDS.md).

## Licença

Distribuído sob a **[licença MIT](LICENSE)**. Consulte o arquivo `LICENSE` para os termos completos.
