# 📖 Referência de Comandos do Caramel CLI

Este documento contém a referência completa de todos os comandos e subcomandos disponíveis no **Caramel CLI**, com suas respectivas sintaxes, flags e exemplos práticos de uso.

---

## 📌 Sumário de Comandos

- [`caramel version`](#1-caramel-version) - Exibe detalhes da versão e compilação.
- [`caramel config`](#2-caramel-config) - Gerencia configurações e chaves de API (OpenRouter).
- [`caramel process`](#3-caramel-process) - **[Pipeline Automatizado]** Extrai, colore e reconstrói o `.docx` de uma só vez (com redimensionamento exato de imagens).
- [`caramel docx extract`](#4-caramel-docx-extract) - Inspeciona, extrai e colore imagens de arquivos `.docx`.
- [`caramel image colorize`](#5-caramel-image-colorize) - Colora ilustrações em preto e branco via IA.
- [`caramel routine process`](#6-caramel-routine-process) - Processa rotinas semanais e compila relatório de Campos de Experiência da BNCC.
- [`caramel install`](#7-caramel-install) - Instala o Caramel CLI globalmente no sistema.

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
2. Ignora automaticamente brasões, logos e ícones pequenos (padrão: `< 20KB`).
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
| `--min-size` | `-s` | string | `20KB` | Tamanho mínimo para processar a imagem (ex: `20KB`, `50KB`, `0` para todas). |
| `--output` | `-o` | string | Dinâmico (`imagens <nome>`) | Diretório onde os arquivos gerados serão salvos. |
| `--model` | `-m` | string | `google/gemini-3.1-flash-image` | Modelo de IA multimodal a ser utilizado no OpenRouter. |
| `--verbose`| `-v` | bool | `false` | Exibe o log raw de depuração da API. |

### Exemplo Prático
```bash
# Processa o arquivo gerando "atividade colorida.docx" com imagens no mesmo layout
caramel process atividade.docx
```

---

## 4. `caramel docx extract`

Extrai todas as imagens de um arquivo `.docx`. Opcionalmente, ativa a IA (`--colorize` / `-c`) para colorir desenhos e ilustrações em preto e branco.

### Sintaxe
```bash
caramel docx extract <caminho-do-arquivo.docx> [flags]
```

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--min-size` | `-s` | string | `20KB` | Tamanho mínimo para extrair a imagem (ex: `20KB`, `50KB`, `0` para todas). |
| `--output` | `-o` | string | Dinâmico (`imagens <nome>`) | Diretório onde as imagens extraídas serão salvas. |
| `--list` | `-l` | bool | `false` | Apenas lista as imagens contidas no arquivo sem extraí-las. |
| `--colorize`| `-c` | bool | `false` | Colora automaticamente as imagens extraídas usando IA. |

---

## 5. `caramel image colorize`

Colora ilustrações em preto e branco (PNG, JPEG, WEBP) usando a IA multimodal do OpenRouter. Suporta imagens individuais, pastas inteiras e **arquivos `.docx`**, apresentando uma galeria com preview ANSI em tempo real no terminal para seleção interativa.

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
| `--output` | `-o` | string | Pasta original ou `<docx>_coloridas` | Diretório onde as imagens coloridas serão salvas. |
| `--model` | `-m` | string | `google/gemini-2.5-flash-image` | Modelo de IA multimodal a ser utilizado no OpenRouter. |
| `--verbose`| `-v` | bool | `false` | Exibe o log de depuração da API. |

### Exemplos Práticos

```bash
# Colora uma única imagem diretamente
caramel colorize desenho.png

# Seleção interativa de imagens contidas em um arquivo .docx
caramel colorize atividade.docx

# Seleção interativa de todas as imagens em uma pasta
caramel colorize ./imagens_pedagogicas/
```

---

## 6. `caramel routine process`

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

## 7. `caramel install`

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


