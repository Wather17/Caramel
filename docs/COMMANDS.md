# 📖 Referência de Comandos do Caramel CLI

Este documento contém a referência completa de todos os comandos e subcomandos disponíveis no **Caramel CLI**, com suas respectivas sintaxes, flags e exemplos práticos de uso.

---

## 📌 Sumário de Comandos

- [`caramel version`](#1-caramel-version) - Exibe detalhes da versão e compilação.
- [`caramel config`](#2-caramel-config) - Gerencia configurações e chaves de API (OpenRouter).
- [`caramel process`](#3-caramel-process) - **[Pipeline Automatizado]** Extrai e colore todas as imagens de um `.docx` de uma só vez (com filtro automático de brasões/logos).
- [`caramel docx extract`](#4-caramel-docx-extract) - Inspeciona, extrai e colore imagens de arquivos `.docx`.
- [`caramel image colorize`](#5-caramel-image-colorize) - Colora ilustrações em preto e branco via IA.

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
2. Ignora automaticamente brasões, logos e ícones pequenos (padrão: `< 20KB`) para não desperdiçar IA.
3. Extrai todas as imagens mantidas para uma pasta com nome higienizado (ex: `./imagens prova de geografia/`)
4. Colora cada imagem automaticamente usando a IA multimodal Nano Banana 2 (`google/gemini-3.1-flash-image`)
5. Exibe o resumo detalhado no terminal.

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
| `--output` | `-o` | string | Dinâmico (`imagens <nome>`) | Diretório onde as imagens coloridas serão salvas. |
| `--model` | `-m` | string | `google/gemini-3.1-flash-image` | Modelo de IA multimodal a ser utilizado no OpenRouter. |
| `--verbose`| `-v` | bool | `false` | Exibe o log raw de depuração da API. |

### Exemplo Prático
```bash
# Processa o arquivo ignorando automaticamente brasões e logos menores que 20KB
caramel process atividade_geografia.docx

# Processa todas as imagens sem filtrar tamanho
caramel process atividade.docx -s 0
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

Colora uma ilustração isolada (PNG, JPEG, WEBP, SVG) em preto e branco usando a IA multimodal da OpenRouter.

### Sintaxe
```bash
caramel image colorize <imagem> [flags]
```

### Aliases (Atalhos)
`caramel colorize <imagem>`, `caramel color <imagem>`

### Exemplo Prático
```bash
caramel colorize desenho_linha.png
```
