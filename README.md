# 🍬 Caramel CLI

**Caramel** é uma ferramenta de linha de comando (CLI) desenvolvida em **Go** para uso diário na criação de atividades e utilitários de desenvolvimento pedagógico.

---

## 🌟 Recursos Principais

- ⚡ **Nativo e Ultra Rápido**: Compilado em binários nativos sem dependências externas.
- 🖼️ **Extrator de Imagens DOCX**: Extração e listagem automática de imagens contidas em arquivos `.docx`.
- 💻 **Multiplataforma**: Executáveis pré-compilados para Linux e Windows.
- 📦 **Instalação Simplificada**: Scripts automatizados de instalação para Linux (`.sh`) e Windows (`.ps1`).
- 📚 **Arquitetura Modular**: Estrutura limpa (Clean Layout) para fácil expansão de novas ferramentas pedagógicas.

---

## 📚 Documentação

Toda a documentação técnica e guias de uso/desenvolvimento estão disponíveis no diretório [`docs/`](docs/):

- 📖 [**Referência de Comandos**](docs/COMMANDS.md): Lista completa de comandos, sintaxe, flags e exemplos práticos.
- 🎨 [**Guia de Estilo & Cores (Design System)**](docs/DESIGN_SYSTEM.md): Paleta de cores, inspiração e boas práticas da TUI.
- 🏛️ [**Arquitetura do Projeto**](docs/ARCHITECTURE.md): Estrutura de pastas, fluxo de dados e compilação.
- 🚀 [**Guia de Início Rápido (Getting Started)**](docs/GETTING_STARTED.md): Como compilar, testar e instalar.
- 🛠️ [**Como Criar Comandos e Ferramentas**](docs/CONTRIBUTING_COMMANDS.md): Guia passo a passo para adicionar novos recursos.

---

## 🚀 Uso Rápido

### Extrair imagens de um arquivo `.docx`
```bash
go run ./cmd/caramel docx extract atividade.docx -o ./imagens_extraidas
```

### Apenas listar imagens sem salvar no disco
```bash
go run ./cmd/caramel docx extract prova.docx --list
```

### Compilar para Linux e Windows
```bash
./scripts/build.sh
```

---

## 📄 Licença

Desenvolvido para auxílio e ferramentas de desenvolvimento pedagógico.
