package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandCategory representa a categoria temática de um comando pedagógico
type CommandCategory string

const (
	CategoryMedia  CommandCategory = "🖼️ Mídia & Arquivos"
	CategoryConfig CommandCategory = "⚙️ Configurações & IA"
	CategorySystem CommandCategory = "ℹ️ Sistema & Utilidades"
)

// ExampleDoc representa um exemplo prático de uso do comando
type ExampleDoc struct {
	Description string // Explicação em linguagem simples e didática
	Command     string // Comando exato para copiar/executar
}

// FlagDoc representa a documentação de uma flag
type FlagDoc struct {
	Flag        string // ex: "-i, --interactive"
	Description string // Explicação simples da utilidade
}

// CommandHelpDoc estrutura a documentação de um comando, derivada AO VIVO da árvore
// de comandos do Cobra — não existe mais lista manual para manter sincronizada.
type CommandHelpDoc struct {
	Name               string          // Caminho do comando (ex: "caramel docx extract")
	Short              string          // Resumo curto
	Category           CommandCategory // Categoria derivada do comando pai
	PedagogicalContext string          // Long do comando (contexto didático de uso)
	Syntax             string          // Sintaxe de uso (UseLine)
	Flags              []FlagDoc       // Flags reais do comando (LocalFlags)
	Examples           []ExampleDoc    // Exemplos parseados do campo Example do Cobra
	Aliases            string          // Aliases do comando (ex: "pipeline, run")
}

// rootCmd guarda a árvore de comandos registrada pelo CLI para alimentar o guia ao vivo
var rootCmd *cobra.Command

// SetRootCommand registra a árvore de comandos Cobra que alimenta o guia didático.
// Deve ser chamado pelo pacote cli durante a inicialização.
func SetRootCommand(cmd *cobra.Command) {
	rootCmd = cmd
}

// GetAllCommandDocs percorre a árvore de comandos e deriva a documentação ao vivo.
// Flags, sintaxe, aliases e exemplos são extraídos dos próprios comandos Cobra,
// eliminando a duplicação manual que causava defasagem entre guia e CLI.
func GetAllCommandDocs() []CommandHelpDoc {
	if rootCmd == nil {
		return nil
	}

	seen := make(map[*cobra.Command]bool)
	var docs []CommandHelpDoc
	collectCommands(rootCmd, seen, &docs)

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs
}

// collectCommands percorre recursivamente a árvore (deduplicando comandos registrados
// em múltiplos pais, ex: 'caramel process' também existe como 'caramel docx process')
func collectCommands(cmd *cobra.Command, seen map[*cobra.Command]bool, docs *[]CommandHelpDoc) {
	for _, sub := range cmd.Commands() {
		if seen[sub] || !sub.IsAvailableCommand() {
			continue
		}

		// Ignora os comandos de autocompletar gerados automaticamente pelo Cobra
		if sub.Name() == "completion" || (sub.Parent() != nil && sub.Parent().Name() == "completion") {
			seen[sub] = true
			continue
		}

		seen[sub] = true

		if sub.Runnable() {
			*docs = append(*docs, buildDoc(sub))
		}
		collectCommands(sub, seen, docs)
	}
}

// buildDoc constrói a documentação de um comando a partir dos seus próprios campos
func buildDoc(cmd *cobra.Command) CommandHelpDoc {
	path := commandDisplayPath(cmd)

	return CommandHelpDoc{
		Name:               path,
		Short:              cmd.Short,
		Category:           categoryFor(cmd),
		PedagogicalContext: cmd.Long,
		Syntax:             cmd.UseLine(),
		Flags:              collectFlagDocs(cmd),
		Examples:           parseExamples(cmd.Example),
		Aliases:            strings.Join(cmd.Aliases, ", "),
	}
}

// commandDisplayPath retorna o nome como o usuário invoca: comandos também registrados
// diretamente no RootCmd (ex: 'caramel process', 'caramel colorize') mostram o caminho curto,
// mesmo que o parent do Cobra seja um grupo (docx/image/pdf)
func commandDisplayPath(cmd *cobra.Command) string {
	if rootCmd != nil {
		for _, child := range rootCmd.Commands() {
			if child == cmd {
				return "caramel " + cmd.Name()
			}
		}
	}
	path := cmd.CommandPath()
	if !strings.HasPrefix(path, "caramel") {
		path = "caramel " + path
	}
	return path
}

// categoryFor deriva a categoria a partir do comando pai da árvore; comandos do topo
// usam um mapa fixo pequeno (estável, não por comando)
func categoryFor(cmd *cobra.Command) CommandCategory {
	if cmd.Parent() != nil && cmd.Parent().Parent() != nil {
		switch cmd.Parent().Name() {
		case "docx", "image", "pdf", "routine":
			return CategoryMedia
		case "config":
			return CategoryConfig
		}
	}

	switch cmd.Name() {
	case "process", "colorize", "generate", "cards", "2up", "extract", "docx":
		return CategoryMedia
	case "config", "setup", "set", "show":
		return CategoryConfig
	}
	return CategorySystem
}

// collectFlagDocs extrai as flags reais do comando (nome, atalho, descrição e padrão)
func collectFlagDocs(cmd *cobra.Command) []FlagDoc {
	var docs []FlagDoc
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}

		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}

		desc := f.Usage
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
			desc += fmt.Sprintf(" (padrão: %s)", f.DefValue)
		}
		docs = append(docs, FlagDoc{Flag: name, Description: desc})
	})
	return docs
}

// parseExamples interpreta o campo Example do Cobra no formato:
//
//	# Descrição do exemplo
//	caramel comando ...
//
//	# Outro exemplo
//	caramel outro-comando ...
func parseExamples(example string) []ExampleDoc {
	var examples []ExampleDoc
	var current *ExampleDoc

	for _, line := range strings.Split(example, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			if current != nil {
				examples = append(examples, *current)
			}
			desc := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			current = &ExampleDoc{Description: desc}
			continue
		}

		if trimmed != "" {
			if current == nil {
				current = &ExampleDoc{}
			}
			if current.Command != "" {
				current.Command += " "
			}
			current.Command += trimmed
		}
	}

	if current != nil {
		examples = append(examples, *current)
	}
	return examples
}