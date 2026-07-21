# 🛠️ Como Criar Novos Comandos e Ferramentas

Este guia explica como adicionar novos subcomandos ao **Caramel CLI** mantendo o código limpo e organizado.

---

## 📌 Passo a Passo para Criar um Novo Comando

### 1. Escreva a Regra de Negócio em `internal/tools/`
Nunca coloque a lógica complexa diretamente no arquivo de CLI. Crie uma função pura no pacote adequado em `internal/tools/`.

**Exemplo**: Criar `internal/tools/activity/quiz.go`:
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

---

### 2. Crie o Comando Cobra em `internal/cli/`
Crie um arquivo para o comando CLI em `internal/cli/`, conectando as flags de linha de comando à função criada no passo 1.

**Exemplo**: Criar `internal/cli/quiz.go`:
```go
package cli

import (
    "fmt"
    "caramel/internal/tools/activity"
    "github.com/spf13/cobra"
)

var (
    topic string
    count int
)

var quizCmd = &cobra.Command{
    Use:   "quiz",
    Short: "Gera um questionário pedagógico rápido",
    RunE: func(cmd *cobra.Command, args []string) error {
        opts := activity.QuizOptions{
            Topic: topic,
            Count: count,
        }
        res, err := activity.GenerateQuiz(opts)
        if err != nil {
            return err
        }
        fmt.Println(res)
        return nil
    },
}

func init() {
    quizCmd.Flags().StringVarP(&topic, "topic", "t", "", "Tópico da atividade (obrigatório)")
    quizCmd.Flags().IntVarP(&count, "count", "c", 5, "Número de questões")
    quizCmd.MarkFlagRequired("topic")

    RootCmd.AddCommand(quizCmd)
}
```

---

### 3. Teste o Novo Comando
Execute no terminal:
```bash
go run ./cmd/caramel quiz --topic "História do Brasil" --count 10
```

Pronto! Seu novo comando está integrado e pronto para compilação multiplataforma!
