# 📖 Referência de Comandos do Caramel CLI

Este documento contém a referência completa de todos os comandos e subcomandos disponíveis no **Caramel CLI**, com suas respectivas sintaxes, flags e exemplos práticos de uso.

---

## 📌 Sumário de Comandos

- [`caramel version`](#1-caramel-version) - Exibe detalhes da versão e compilação.
- [`caramel config`](#2-caramel-config) - Gerencia configurações e chaves de API (OpenRouter).
- [`caramel process`](#3-caramel-process) - **[Pipeline Automatizado]** Extrai, colore e reconstrói o `.docx` de uma só vez (com redimensionamento exato de imagens).
- [`caramel docx extract`](#4-caramel-docx-extract) - Inspeciona, extrai e colore imagens de arquivos `.docx`.
- [`caramel image colorize`](#5-caramel-image-colorize) - Colora ilustrações em preto e branco via IA.
- [`caramel image generate`](#6-caramel-image-generate-harness-em-lote) - **[Harness em Lote]** Gera ilustrações e coleções de objetos pedagógicos com concorrência adaptativa.
- [`caramel cards`](#7-caramel-cards-layout-a4-de-fichas--flashcards) - **[Layout A4 / Flashcards]** Diagrama fichas pedagógicas com legendas para impressão e corte.
- [`caramel routine process`](#8-caramel-routine-process) - Processa rotinas semanais e compila relatório de Campos de Experiência da BNCC.
- [`caramel install`](#9-caramel-install) - Instala o Caramel CLI globalmente no sistema.

---

## 1. `caramel version`

Exibe a versão atual do executável, hash do commit e data de compilação.

```bash
caramel version
```

---

## 2. `caramel config`

Gerencia as configurações e a chave de API do OpenRouter salva em `~/.config/caramel/.env` (ou `%APPDATA%\caramel\.env` no Windows).

### Subcomandos

#### `caramel config setup`
Inicia um assistente interativo no terminal para configurar a chave de API.
```bash
caramel config setup
```

#### `caramel config set <CHAVE> <VALOR>`
Define manualmente o valor de uma chave.
```bash
caramel config set openrouter_key "sk-or-v1-sua-chave-aqui"
```

#### `caramel config show`
Exibe a localização do arquivo de configuração e o status das chaves salvas.
```bash
caramel config show
```

---

## 3. 🚀 `caramel process` (Pipeline Automatizado)

Executa o fluxo fim-a-fim automatizado para um arquivo `.docx`:
1. Inspeciona o arquivo `.docx`
2. Processa todas as imagens extraídas (padrão) — a triagem de economia descarta automaticamente brasões, logos, ícones já coloridos e conteúdos de texto sem custo de API.
3. Extrai todas as imagens mantidas para uma pasta temporária.
4. Colora cada imagem automaticamente usando a IA multimodal Nano Banana 2 (`google/gemini-3.1-flash-image`).
5. **Redimensiona as imagens geradas pela IA** para baterem exatamente com a resolução em pixels da original, evitando distorções (aspect ratio) de layout.
6. **Reconstrói e gera um novo arquivo `.docx`** (ex: `atividade colorida.docx`) com as imagens coloridas no mesmo local.

### Sintaxe
```bash
caramel process <caminho-do-arquivo.docx> [flags]
```

### Aliases (Atalhos)
`caramel pipeline <arquivo.docx>`, `caramel run <arquivo.docx>`, `caramel docx process <arquivo.docx>`

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--min-size` | `-s` | string | `0` | Tamanho mínimo para processar a imagem (ex: `20KB`, `50KB`, `0` para todas). |
| `--interactive` | `-i` | bool | `false` | Habilita modo interativo com preview ANSI TrueColor no terminal. |
| `--output` | `-o` | string | `imagens <nome_do_arquivo>` | Diretório de destino do novo `.docx` e das imagens. |
| `--model` | `-m` | string | `google/gemini-3.1-flash-image-preview` | Modelo de IA multimodal a ser utilizado no OpenRouter. |
| `--triage-model` | | string | `qwen/qwen3.7-flash` | Modelo de visão da triagem de economia ($0.03/M input). |
| `--no-triage` | | bool | `false` | Desativa a triagem e colore todas as imagens elegíveis diretamente. |
| `--verbose`| `-v` | bool | `false` | Exibe o log de depuração da API. |

### Triagem de Economia (padrão ativada)

Antes de enviar cada imagem para a coloração (etapa paga), o Caramel executa uma triagem em duas camadas:

1. **Análise local (custo zero):** mede a saturação de cor da imagem — se ela já estiver colorida, é pulada imediatamente.
2. **Modelo de visão de baixo custo (Qwen 3.7 Flash, $0.03/M input):** decide se a imagem em P&B é uma ilustração colorível ou apenas texto/tabela/diagrama.

Imagens reprovadas são **puladas** (não são substituídas no `.docx`) e reportadas no resumo final.
Em caso de erro na triagem (ex: rate limit), a imagem é colorida normalmente (**fail-open**) — nenhuma ilustração legítima é perdida.

### Exemplos Prático
```bash
# Processa de forma totalmente automatizada
caramel process atividade.docx

# Modo interativo selecionando imagens individualmente com preview ANSI
caramel process atividade.docx -i
```

---

## 4. `caramel docx extract`

Extrai todas as imagens de um arquivo `.docx`. Opcionalmente, ativa a IA (`--colorize` / `-c`) para colorir desenhos e ilustrações em preto e branco.

### Sintaxe
```bash
caramel docx extract <arquivo.docx> [flags]
```

### Aliases (Atalhos)
`caramel extract <arquivo.docx>`

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--output` | `-o` | string | `imagens <nome_do_arquivo>` | Diretório onde as imagens serão salvas. |
| `--list` | `-l` | bool | `false` | Apenas lista as imagens no terminal sem salvar. |
| `--interactive` | `-i` | bool | `false` | Abre formulário interativo de seleção de imagens. |
| `--colorize`| `-c` | bool | `false` | Colora automaticamente as imagens extraídas usando IA. |
| `--min-size` | `-s` | string | `0` | Tamanho mínimo da imagem para ser extraída (ex: `20KB`, `50KB`, `0` para todas). |
| `--model` | `-m` | string | `google/gemini-3.1-flash-image-preview` | Modelo de IA multimodal a ser utilizado no OpenRouter. |
| `--triage-model` | | string | `qwen/qwen3.7-flash` | Modelo de visão da triagem de economia ($0.03/M input). |
| `--no-triage` | | bool | `false` | Desativa a triagem e colore todas as imagens elegíveis diretamente. |

---

## 5. `caramel image colorize`

Colora ilustrações em preto e branco (PNG, JPEG, WEBP) usando a IA multimodal do OpenRouter. Suporta imagens individuais, pastas inteiras e **arquivos `.docx`**, apresentando uma galeria com preview ANSI em tempo real no terminal para seleção interativa.

Quando executado sobre um arquivo `.docx`, executa o pipeline unificado: extrai, colore via IA, redimensiona para preservar proporções e reconstrói o arquivo `<nome> colorida.docx` além de salvar as imagens coloridas.

### Sintaxe
```bash
caramel colorize <imagem | pasta | arquivo.docx> [flags]
```

### Aliases (Atalhos)
`caramel colorize <alvo>`, `caramel color <alvo>`, `caramel image colorize <alvo>`

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--interactive` | `-i` | bool | `false` | Habilita seleção interativa com miniaturas ANSI no terminal. |
| `--all` | `-a` | bool | `false` | Colora todas as imagens sem abrir formulário de seleção. |
| `--min-size` | `-s` | string | `0` | Tamanho mínimo da imagem ao processar `.docx` (ex: `20KB`, `50KB`, `0` para todas). |
| `--output` | `-o` | string | Pasta original ou pasta de imagens do docx | Diretório onde os arquivos/imagens serão salvos. |
| `--model` | `-m` | string | `google/gemini-2.5-flash-image` | Modelo de IA multimodal a ser utilizado no OpenRouter. |
| `--triage-model` | | string | `qwen/qwen3.7-flash` | Modelo de visão da triagem de economia ($0.03/M input). |
| `--no-triage` | | bool | `false` | Desativa a triagem e colore todas as imagens selecionadas diretamente. |
| `--verbose`| `-v` | bool | `false` | Exibe o log de depuração da API. |

### Exemplos Práticos

```bash
# Colora uma única imagem diretamente
caramel colorize desenho.png

# Processa e reconstrói o arquivo .docx com imagens coloridas
caramel colorize atividade.docx

# Seleção interativa com preview ANSI das imagens contidas no .docx
caramel colorize atividade.docx -i

# Seleção interativa de todas as imagens em uma pasta
caramel colorize ./imagens_pedagogicas/
```

---

## 6. `caramel image generate` (Harness em Lote)

Gera coleções visuais de ilustrações e objetos pedagógicos a partir de listas de palavras ou temas.
A IA sintetiza e padroniza os prompts visuais e um motor com concorrência adaptativa gera as imagens em alta velocidade sem bloqueios de rate limit.

### Sintaxe
```bash
caramel generate <itens | flags>
caramel image generate [flags]
```

### Aliases (Atalhos)
`caramel generate`, `caramel gen`, `caramel img gen`

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--items` | `-i` | string | `""` | Lista de palavras ou termos separados por vírgula. |
| `--file` | `-f` | string | `""` | Caminho para arquivo de texto contendo itens linha por linha. |
| `--theme` | `-t` | string | `""` | Tema para a IA escolher os itens automaticamente (ex: "animais da fazenda"). |
| `--count` | `-n` | int | `10` | Quantidade de itens a gerar quando utilizado com `--theme`. |
| `--style` | `-s` | string | `clipart` | Estilo: `clipart`, `vector`, `3d-cute`, `coloring`, `realistic`. |
| `--2up` | | bool | `false` | Compila automaticamente todas as imagens geradas em um PDF 2-up A4. |
| `--preview` | | bool | `true` | Exibe miniaturas ANSI TrueColor no terminal durante a geração. |
| `--output` | `-o` | string | `./imagens_<tema>` | Diretório onde as imagens serão salvas. |
| `--workers`| `-w` | int | `0` (adaptativo) | Quantidade de workers concorrentes manuais. |

### Exemplos Práticos

```bash
# Gerar ilustrações a partir de uma lista
caramel generate --items "maçã, banana, melancia, uva" -s clipart

# Gerar 10 animais de um tema e compilar diretamente em PDF 2-up para impressão
caramel generate --theme "animais da savana" -n 10 -s 3d-cute --2up

# Gerar desenhos para colorir a partir de um arquivo de texto
caramel generate -f ./frutas.txt -s coloring
```

---

## 7. `caramel cards` (Layout A4 de Fichas / Flashcards)

Diagrama coleções de imagens em folhas de papel A4 prontas para impressão e recorte, com legendas em caixa alta e linhas tracejadas de tesoura ✂️.

### Sintaxe
```bash
caramel cards <pasta_ou_imagem> [flags]
```

### Aliases (Atalhos)
`caramel flashcards`, `caramel print cards`

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--cols` | `-c` | int | `2` | Número de colunas por folha A4 (ex: 2, 3 ou 4). |
| `--rows` | `-r` | int | `3` | Número de linhas por folha A4 (ex: 2, 3 ou 4). |
| `--title` | `-t` | string | `""` | Título no cabeçalho de cada folha. |
| `--cut-lines`| `-l` | bool | `true` | Exibe linhas tracejadas de corte com tesoura. |
| `--uppercase`| `-u` | bool | `true` | Exibe os nomes das fichas em caixa alta (ótimo para alfabetização). |
| `--embed` | `-e` | bool | `true` | Embute as imagens em Base64 (arquivo 100% autossuficiente). |
| `--output` | `-o` | string | `<pasta>_fichas_a4.html` | Caminho do arquivo HTML de saída. |

### Exemplos Práticos

```bash
# Gerar fichas A4 de uma pasta de imagens (padrão 2x3 = 6 fichas por folha)
caramel cards ./imagens_frutas/

# Gerar grade 3x3 (9 fichas por folha) com título
caramel cards ./animais/ -c 3 -r 3 -t "Coleção da Fazenda"
```

---

## 8. `caramel routine process`

Lê rotinas pedagógicas semanais em formato `.docx`, extrai todo o texto localmente de forma otimizada, envia para a IA classificar as experiências baseadas na BNCC e reconstrói o relatório final no padrão de tabela Arial em orientação Paisagem.

### Sintaxe
```bash
caramel routine process <caminho_da_pasta_ou_arquivo.docx> [flags]
```

### Aliases (Atalhos)
`caramel routine run <pasta_ou_arquivo>`, `caramel routine pipeline <pasta_ou_arquivo>`

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--output` | `-o` | string | Mesma pasta do arquivo | Diretório onde o arquivo final consolidado será salvo. |
| `--model` | `-m` | string | `google/gemini-2.5-flash` | Modelo de IA para processamento de texto no OpenRouter. |
| `--prompt` | `-p` | string | Nativamente embarcado | Caminho para arquivo com prompt de IA customizado. |
| `--verbose`| `-v` | bool | `false` | Exibe logs raw da API de IA. |

### Exemplo Prático
```bash
# Processa todas as rotinas semanais de uma pasta e gera um relatório mensal consolidado
caramel routine process ./abril/
```

---

## 9. `caramel install`

Copia o binário do Caramel em execução para um diretório local do usuário e adiciona esse diretório ao PATH do sistema operacional de forma totalmente automatizada.

### Sintaxe
```bash
caramel install
```

### Aliases (Atalhos)
`caramel self-install`

### Exemplo Prático
```bash
# Executa o auto-instalador e configura o PATH global
caramel install
```


