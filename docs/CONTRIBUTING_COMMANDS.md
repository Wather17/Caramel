# 🛠️ Como Criar Novos Comandos e Ferramentas

Este guia define **as políticas oficiais** para adicionar novos subcomandos ao **Caramel CLI**:
nomenclatura, flags, documentação e estrutura de código.

> **Importante:** o guia didático (`caramel guide`) é **gerado ao vivo** a partir da árvore de
> comandos Cobra — `Short`, `Long`, `Example`, aliases e flags são exibidos sem nenhuma lista
> manual. Siga as políticas abaixo para que o guia, o `--help` e a busca continuem consistentes
> automaticamente.

---

## 🏗️ Estrutura de um Novo Comando

### 1. Regra de negócio em `internal/tools/`

Nunca coloque a lógica complexa no arquivo de CLI. Crie uma função pura no pacote adequado de `internal/tools/`.

**Exemplo**: criar `internal/tools/activity/quiz.go`:

```go
package activity

type QuizOptions struct {
	Topic string
	Count int
}

func GenerateQuiz(opts QuizOptions) (string, error) {
	// Lógica para gerar quiz pedagógico...
	return "Quiz gerado com sucesso!", nil
}
```

### 2. Comando Cobra em `internal/cli/`

Crie um arquivo para o comando em `internal/cli/`, conectando as flags à função do passo 1.
Siga o padrão de um arquivo por comando, com as flags declaradas como variáveis de pacote:

```go
package cli

import (
	"fmt"

	"caramel/internal/tools/activity"

	"github.com/spf13/cobra"
)

var (
	quizTopic string
	quizCount int
)

var quizCmd = &cobra.Command{
	Use:     "quiz",
	Aliases: []string{"perguntas"},
	Short:   "Gera um questionário pedagógico rápido",
	Long: `Gera um questionário com N perguntas sobre um tema, pronto para impressão.

📚 QUANDO USAR:
Use para montar avaliações rápidas ou revisões de conteúdo em sala. O tema é
obrigatório; a quantidade de perguntas é configurável com --count.`,
	Example: `# Gerar um quiz de 10 perguntas sobre a História do Brasil
caramel activity quiz --topic "História do Brasil" --count 10`,
	Args: cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := activity.GenerateQuiz(activity.QuizOptions{
			Topic: quizTopic,
			Count: quizCount,
		})
		if err != nil {
			return err
		}
		fmt.Println(res)
		return nil
	},
}

func init() {
	quizCmd.Flags().StringVarP(&quizTopic, "topic", "t", "", "Tópico da atividade (obrigatório)")
	quizCmd.Flags().IntVarP(&quizCount, "count", "c", 5, "Número de questões")
	quizCmd.MarkFlagRequired("topic")

	activityCmd.AddCommand(quizCmd)
}
```

### 3. Registre no grupo e teste

- Registre o comando no **grupo pedagógico** apropriado (ver [política de nomes](#-política-de-nomes)).
- Atalho na raiz: **somente** para compatibilidade, com um comentário indicando o motivo.
- Teste com `go build ./...`, `go vet ./...`, `go test ./...` e o smoke `caramel guide <termo>`.

---

## 📝 Política de Nomes

| Regra | Exemplo |
| :--- | :--- |
| **Inglês** (mais prático de digitar), minúsculas, verbo | `extract`, `colorize`, `generate` |
| **Palavra única** (nunca com espaço) | `2up` é aceitável; `print cards` é proibido |
| **Comando vive num grupo** por fluxo pedagógico | `docx`, `image`, `print`, `routine` |
| **Raiz só para o que não pertence a fluxo** | `guide`, `install`, `version` |
| **Aliases em português** são bem-vindos | `colorir`, `ajuda` |
| **Aliases sem colisão** (outros comandos/grupos) | `pipeline`/`run` duplicados é o anti-padrão |

### Grupos de fluxo pedagógico existentes

| Grupo | Finalidade | Comandos |
| :--- | :--- | :--- |
| `docx` | Manipulação de documentos Word | `extract` |
| `image` | Criação e tratamento de imagens | `colorize`, `generate` |
| `print` | Preparação de materiais para impressão | `2up`, `cards` |
| `routine` | Rotinas de aula | `process` |
| `config` | Configuração de chaves e preferências | `set`, `show`, `setup` |

### Atalho silencioso na raiz (compat)

Comandos legados que os usuários já usam na raiz podem ser registrados **também** no
`RootCmd`, mas nunca aparecem no guia com esse caminho (o guia mostra só o caminho agrupado):

```go
printCmd.AddCommand(cardsCmd)

// Atalho silencioso no RootCmd para compatibilidade com 'caramel cards'
RootCmd.AddCommand(cardsCmd)
```

Ao absorver um comando antigo, mova os aliases legados para o novo comando e filtre-os do
guia em `legacyAliases` em `internal/ui/help_data.go` (ex: `process` virou alias de `colorize`).

---

## 🚩 Política de Flags

### Shorthands padronizados (quando a flag existir)

| Shorthand | Significado |
| :--- | :--- |
| `-o` | `--output` (diretório/arquivo de saída) |
| `-m` | `--model` (modelo de IA) |
| `-v` | `--verbose` (logs de depuração) |
| `-i` | `--interactive` (seleção interativa) |
| `-s` | `--min-size` (tamanho mínimo de arquivo) |

### Regras

1. **Bool com default `true` exige off-switch `--no-*`.** Toda flag ligada por padrão
   (`--optimize`, `--cards`, `--preview`, `--uppercase`, `--auto-rotate`, `--duplicate`)
   precisa de um `--no-...` correspondente. Nada de botão sem como desligar:

   ```go
   cmd.Flags().BoolVarP(&opt, "optimize", "O", true, "Redimensiona e comprime imagens pesadas")
   cmd.Flags().BoolVar(&opt, "no-optimize", false, "Desativa a compactação/redimensionamento")
   opt = true // restaura o padrão (pflag sobrescreve a variável ao registrar a negativa)
   ```

2. **Negativas usam o prefixo `--no-*`** (`--no-triage`, `--no-cards`, `--no-preview`).
   Nunca inverta o significado de uma flag existente.

3. **Descrição de 1 linha em português**, sem ambiguidade entre flags que parecem sinônimas.
   Diferencie claramente o que cada uma controla (ex: `-a/--all` controla o formulário de
   seleção; `--no-triage` controla a triagem de economia).

4. **Valide valores na flag** com whitelist ou faixa (ex: `--fit contain|cover`, `--aspect`,
   grade de `cards` de 1 a 6) e retorne erro amigável.

5. **Evite flags "ação que já é padrão"**: se algo já acontece por padrão, a flag negativa
   (`--no-*`) é a forma de desligar — nunca uma flag positiva duplicada.

---

## 📚 Política de Documentação (obrigatória por comando)

| Campo | Regra |
| :--- | :--- |
| `Short` | Resumo de **até ~60 caracteres**, em português, começando com verbo |
| `Long` | Contexto pedagógico com seção **`📚 QUANDO USAR`** (para que serve, quando usar) |
| `Example` | **Exemplos reais no campo `Example`** do Cobra, com `# descrição` antes de cada comando |
| `--help` | Proibido colocar `EXEMPLOS:` dentro do `Long` — o campo `Example` já renderiza |

Formato do campo `Example` (parseado pelo guia para a busca):

```go
Example: `# Explicação didática do exemplo
caramel <grupo> <comando> ...`,
```

Use sempre o **caminho agrupado** nos exemplos (`caramel print 2up`, `caramel image generate`),
que é o que o guia ensina.

> Como o guia é gerado ao vivo, **nunca** edite `docs/COMMANDS.md` manualmente para listar
> comandos novos — atualize apenas se precisar documentar política/organização.

---

## ✅ Checklist antes de abrir PR

- [ ] Lógica em `internal/tools/`, comando em `internal/cli/`
- [ ] Comando registrado no grupo pedagógico correto (atalho raiz só com comentário de compat)
- [ ] `Short`, `Long` (com `📚 QUANDO USAR`) e `Example` preenchidos
- [ ] Flags seguem os shorthands padronizados e off-switches `--no-*`
- [ ] Aliases sem colisão e sem espaço; legados adicionados em `legacyAliases`
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` passando
- [ ] Smoke: `caramel guide`, `caramel guide <termo>`, `caramel <comando> --help`