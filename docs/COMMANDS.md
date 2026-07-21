# 📖 Referência de Comandos do Caramel CLI

Este documento contém a referência completa de todos os comandos e subcomandos disponíveis no **Caramel CLI**, com suas respectivas sintaxes, flags e exemplos práticos de uso.

---

## 📌 Sumário de Comandos

- [`caramel version`](#1-caramel-version) - Exibe detalhes da versão e compilação.
- [`caramel config`](#2-caramel-config) - Gerencia configurações e chaves de API (OpenRouter).
- [`caramel docx extract`](#3-caramel-docx-extract) - Inspeciona, extrai e colore imagens de arquivos `.docx`.
- [`caramel image colorize`](#4-caramel-image-colorize) - Colora ilustrações em preto e branco via IA.

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

## 3. `caramel docx extract`

Extrai todas as imagens de um arquivo `.docx`. Opcionalmente, ativa a IA (`--colorize` / `-c`) para colorir desenhos e ilustrações em preto e branco usando o modelo `google/nano-banana-2` via OpenRouter.

### Sintaxe
```bash
caramel docx extract <caminho-do-arquivo.docx> [flags]
```

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--output` | `-o` | string | Dinâmico (`imagens <nome>`) | Diretório onde as imagens extraídas serão salvas. |
| `--list` | `-l` | bool | `false` | Apenas lista as imagens contidas no arquivo sem extraí-las. |
| `--colorize`| `-c` | bool | `false` | Colora automaticamente as imagens extraídas usando IA. |
| `--model` | `-m` | string | `google/nano-banana-2` | Modelo de IA multimodal do OpenRouter a ser utilizado. |

### Exemplos Práticos

#### Extrair e colorir imagens automaticamente via IA
```bash
caramel docx extract atividade_geografia.docx --colorize
```

---

## 4. `caramel image colorize`

Colora uma ilustração isolada (PNG, JPEG, WEBP, SVG) em preto e branco usando a IA multimodal da OpenRouter.

### Sintaxe
```bash
caramel image colorize <imagem> [flags]
```

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--output` | `-o` | string | Pasta da imagem original | Diretório onde a imagem colorida será salva. |
| `--model` | `-m` | string | `google/nano-banana-2` | Modelo de IA a ser utilizado no OpenRouter. |

### Exemplo Prático
```bash
caramel image colorize desenho_linha.png -o ./imagens_coloridas
```
