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
	CategoryDocx    CommandCategory = "📄 Documentos Word (.docx)"
	CategoryImage   CommandCategory = "🎨 Imagens"
	CategoryPrint   CommandCategory = "🖨️ Impressão & Papel"
	CategoryRoutine CommandCategory = "📅 Rotinas de Aula"
	CategoryConfig  CommandCategory = "⚙️ Configurações & IA"
	CategorySystem  CommandCategory = "ℹ️ Sistema & Utilidades"
)

// legacyAliases são apelidos mantidos apenas por compatibilidade (ex: 'process'
// foi absorvido pelo 'colorize'). Não são exibidos no guia para não poluir a navegação.
var legacyAliases = map[string]bool{
	"process":  true,
	"pipeline": true,
	"run":      true,
}

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
//
// A exibição prioriza o caminho agrupado por fluxo pedagógico ('caramel print 2up',
// 'caramel image colorize'...). Comandos registrados também na raiz apenas por
// compatibilidade ('caramel 2up', 'caramel process'...) não aparecem na raiz.
func GetAllCommandDocs() []CommandHelpDoc {
	if rootCmd == nil {
		return nil
	}

	seen := make(map[*cobra.Command]bool)
	var docs []CommandHelpDoc

	// Fase 1: comandos dentro de grupos pedagógicos (caminho agrupado)
	for _, child := range rootCmd.Commands() {
		if child == nil || seen[child] || !child.IsAvailableCommand() || child.Runnable() {
			continue
		}
		if child.Name() == "completion" {
			seen[child] = true
			continue
		}
		collectGroupLeaves(child, seen, &docs)
	}

	// Fase 2: comandos autênticos da raiz (guide, install, version, config...)
	for _, child := range rootCmd.Commands() {
		if child == nil || seen[child] || !child.IsAvailableCommand() || !child.Runnable() {
			continue
		}
		if child.Name() == "completion" {
			seen[child] = true
			continue
		}
		seen[child] = true
		docs = append(docs, buildDoc(child, "caramel "+child.Name(), categoryForRoot(child)))
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs
}

// collectGroupLeaves percorre recursivamente um grupo e documenta suas folhas com
// o caminho agrupado (ex: 'caramel print 2up'), deduplicando comandos que também
// são registrados na raiz para compatibilidade.
func collectGroupLeaves(group *cobra.Command, seen map[*cobra.Command]bool, docs *[]CommandHelpDoc) {
	for _, sub := range group.Commands() {
		if sub == nil || seen[sub] || !sub.IsAvailableCommand() {
			continue
		}

		// Ignora os comandos de autocompletar gerados automaticamente pelo Cobra
		if sub.Name() == "completion" || (sub.Parent() != nil && sub.Parent().Name() == "completion") {
			seen[sub] = true
			continue
		}

		seen[sub] = true

		if sub.Runnable() {
			path := group.CommandPath() + " " + sub.Name()
			*docs = append(*docs, buildDoc(sub, path, categoryForGroup(group)))
		}
		collectGroupLeaves(sub, seen, docs)
	}
}

// buildDoc constrói a documentação de um comando a partir dos seus próprios campos
func buildDoc(cmd *cobra.Command, path string, category CommandCategory) CommandHelpDoc {
	return CommandHelpDoc{
		Name:               path,
		Short:              cmd.Short,
		Category:           category,
		PedagogicalContext: cmd.Long,
		Syntax:             cmd.UseLine(),
		Flags:              collectFlagDocs(cmd),
		Examples:           parseExamples(cmd.Example),
		Aliases:            strings.Join(filteredAliases(cmd.Aliases), ", "),
	}
}

// filteredAliases remove apelidos de compatibilidade legados (ex: 'process', 'run')
func filteredAliases(aliases []string) []string {
	var out []string
	for _, a := range aliases {
		if !legacyAliases[a] {
			out = append(out, a)
		}
	}
	return out
}

// categoryForGroup deriva a categoria do grupo pedagógico que contém o comando
func categoryForGroup(group *cobra.Command) CommandCategory {
	switch group.Name() {
	case "docx":
		return CategoryDocx
	case "image":
		return CategoryImage
	case "print":
		return CategoryPrint
	case "routine":
		return CategoryRoutine
	case "config":
		return CategoryConfig
	}
	return CategorySystem
}

// categoryForRoot deriva a categoria de comandos autênticos da raiz (guide, install, version...)
func categoryForRoot(cmd *cobra.Command) CommandCategory {
	switch cmd.Name() {
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