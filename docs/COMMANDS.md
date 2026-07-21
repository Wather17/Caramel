# 📖 Referência de Comandos do Caramel CLI

Este documento contém a referência completa de todos os comandos e subcomandos disponíveis no **Caramel CLI**, com suas respectivas sintaxes, flags e exemplos práticos de uso.

---

## 📌 Sumário de Comandos

- [`caramel version`](#1-caramel-version) - Exibe detalhes da versão e compilação.
- [`caramel docx extract`](#2-caramel-docx-extract) - Inspeciona e extrai imagens de arquivos `.docx`.

---

## 1. `caramel version`

Exibe a versão atual do executável, hash do commit e data de compilação.

### Sintaxe
```bash
caramel version
```

### Exemplo de Saída
```text
🍬 Caramel CLI v0.1.0-dev
   Commit: 1bbc196
   Data de Build: 2026-07-21T11:13:56Z
```

---

## 2. `caramel docx extract`

Extrai todas as imagens (diagramas, fotos, gráficos) contidas dentro de um arquivo de documento `.docx` de forma automática, salvando-as em uma pasta de destino no disco.

Por padrão, o Caramel cria uma pasta com **nome dinâmico e higienizado usando espaços** baseado no nome do arquivo original (ex: `atividade_historia.docx` -> `./imagens atividade historia/`).

### Sintaxe
```bash
caramel docx extract <caminho-do-arquivo.docx> [flags]
```

### Argumentos
- `<caminho-do-arquivo.docx>` (Obrigatório): Caminho para o arquivo `.docx` a ser analisado.

### Flags Disponíveis

| Flag | Atalho | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `--output` | `-o` | string | Dinâmico (`imagens <nome>`) | Diretório onde as imagens extraídas serão salvas. |
| `--list` | `-l` | bool | `false` | Apenas lista as imagens contidas no arquivo sem extraí-las para o disco. |

---

### 🛡️ Tratamento e Higienização de Nomes de Pastas
Para evitar erros de sistema operacional com arquivos de nomes "sujos" ou muito extensos, o Caramel aplica as seguintes regras automáticas ao gerar o nome padrão da pasta:
1. **Remoção de acentos e símbolos**: `PROVA DE HISTÓRIA 8º ANO (1º SEMESTRE) - CÓPIA (1).docx` -> `imagens prova de historia 8 ano 1 semestre copia 1`
2. **Limite de tamanho de caracteres**: Corta o nome em no máximo 45 caracteres para evitar caminhos corrompidos no sistema operacional.
3. **Uso de espaços legíveis**: Mantém os nomes limpos e legíveis com espaços simples entre as palavras.

---

### Exemplos Práticos

#### 1. Extrair imagens com nome de pasta dinâmico padrão
```bash
caramel docx extract "PROVA DE GEOGRAFIA DA AMÉRICA DO SUL.docx"
# Criará automaticamente a pasta: ./imagens prova de geografia da america do sul/
```

#### 2. Extrair imagens para uma pasta personalizada
```bash
caramel docx extract prova.docx -o "./minhas imagens"
```

#### 3. Apenas listar imagens contidas no documento (sem salvar no disco)
```bash
caramel docx extract simulado.docx --list
```
