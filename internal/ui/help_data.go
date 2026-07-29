package ui

// CommandCategory representa a categoria temática de um comando pedagógico
type CommandCategory string

const (
	CategoryDocx   CommandCategory = "🖼️ Mídia & Arquivos DOCX"
	CategoryConfig CommandCategory = "⚙️ Configurações & Chaves de IA"
	CategorySystem CommandCategory = "ℹ️ Sistema & Ajuda"
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

// CommandHelpDoc estrutura toda a documentação didática de um comando
type CommandHelpDoc struct {
	Name               string          // Nome do comando (ex: "caramel docx extract")
	Short              string          // Resumo curto
	Category           CommandCategory // Categoria
	PedagogicalContext string          // Explicação didática de como/quando usar no dia a dia pedagógico
	Syntax             string          // Sintaxe de uso
	Flags              []FlagDoc       // Lista de flags
	Examples           []ExampleDoc    // Lista de exemplos práticos
}

// GetAllCommandDocs retorna a central de documentação didática de todos os comandos do Caramel
func GetAllCommandDocs() []CommandHelpDoc {
	return []CommandHelpDoc{
		{
			Name:     "caramel process",
			Short:    "Pipeline completo: extrai, colore imagens em P&B com IA e gera novo DOCX colorida",
			Category: CategoryDocx,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize este comando para transformar avaliações, apostilas ou atividades em preto e branco (P&B)
em documentos totalmente coloridos e atrativos para os alunos.

O Caramel irá:
 1. Extrair todas as ilustrações/diagramas do arquivo Word (.docx);
 2. Filtrar automaticamente cabeçalhos e logos pequenos (com base no tamanho);
 3. Colorir cada ilustração utilizando Inteligência Artificial (OpenRouter);
 4. Reconstruir um novo arquivo .docx (ex: 'prova_colorida.docx') pronto para impressão ou uso digital.`,
			Syntax: "caramel process <arquivo.docx> [flags]",
			Flags: []FlagDoc{
				{Flag: "-i, --interactive", Description: "Abre o menu interativo com checkboxes para você escolher visualmente quais imagens deseja colorir."},
				{Flag: "-s, --min-size", Description: "Tamanho mínimo das imagens a serem processadas (ex: '20KB', '50KB'). Ignora logos/brasões."},
				{Flag: "-m, --model", Description: "Modelo de IA do OpenRouter para coloração (padrão: google/gemini-2.5-flash-image)."},
				{Flag: "-o, --output", Description: "Diretório personalizado onde as imagens intermediárias serão salvas."},
				{Flag: "-v, --verbose", Description: "Exibe detalhes técnicos de execução e depuração no terminal."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Processar um arquivo .docx de forma 100% automatizada:",
					Command:     "caramel process avaliacao_historia.docx",
				},
				{
					Description: "Escolher interativamente quais imagens colorir antes de recriar o arquivo:",
					Command:     "caramel process apostila_ciencias.docx -i",
				},
				{
					Description: "Processar ignorando apenas imagens menores que 50KB:",
					Command:     "caramel process livro_exercicios.docx -s 50KB",
				},
			},
		},
		{
			Name:     "caramel docx extract",
			Short:    "Extrai, lista ou colore imagens contidas em um arquivo .docx",
			Category: CategoryDocx,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize este comando se você precisa apenas extrair as figuras contidas em um documento Word
para reaproveitá-las em outro material (apresentações de slides, provas ou atividades no Google Classroom).

Você também pode apenas listar as imagens para inspecionar o conteúdo sem extrair nada para o disco.`,
			Syntax: "caramel docx extract <arquivo.docx> [flags]",
			Flags: []FlagDoc{
				{Flag: "-l, --list", Description: "Apenas inspeciona e exibe a lista de imagens do arquivo sem salvá-las."},
				{Flag: "-i, --interactive", Description: "Abre o menu interativo para você selecionar apenas as figuras que deseja salvar."},
				{Flag: "-c, --colorize", Description: "Ativa a IA para colorir as imagens extraídas."},
				{Flag: "-s, --min-size", Description: "Define o tamanho mínimo para filtrar figuras (padrão: '20KB')."},
				{Flag: "-o, --output", Description: "Diretório de saída para salvar as imagens."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Apenas listar as imagens contidas na prova de Geografia:",
					Command:     "caramel docx extract prova_geografia.docx --list",
				},
				{
					Description: "Extrair todas as imagens para uma pasta específica:",
					Command:     "caramel docx extract atividade.docx -o ./imagens_atividade",
				},
				{
					Description: "Selecionar visualmente quais imagens extrair:",
					Command:     "caramel docx extract mapa_biologia.docx -i",
				},
			},
		},
		{
			Name:     "caramel config setup",
			Short:    "Assistente interativo de configuração de chaves de API",
			Category: CategoryConfig,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize este assistente no primeiro uso do Caramel para cadastrar sua chave de API do OpenRouter.
Ele guia você passo a passo e salva as credenciais com segurança no seu sistema.`,
			Syntax: "caramel config setup",
			Flags:  []FlagDoc{},
			Examples: []ExampleDoc{
				{
					Description: "Iniciar o assistente guiado de configuração:",
					Command:     "caramel config setup",
				},
			},
		},
		{
			Name:     "caramel config show",
			Short:    "Exibe a localização do arquivo de configuração e o status das chaves",
			Category: CategoryConfig,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize para verificar se sua chave do OpenRouter está ativa e onde o arquivo .env de configuração está salvo.`,
			Syntax: "caramel config show",
			Flags:  []FlagDoc{},
			Examples: []ExampleDoc{
				{
					Description: "Verificar o status atual das configurações:",
					Command:     "caramel config show",
				},
			},
		},
		{
			Name:     "caramel config set",
			Short:    "Define o valor de uma chave de configuração (ex: openrouter_key)",
			Category: CategoryConfig,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize para definir diretamente uma chave de configuração sem passar pelo assistente interativo.`,
			Syntax: "caramel config set <CHAVE> <VALOR>",
			Flags:  []FlagDoc{},
			Examples: []ExampleDoc{
				{
					Description: "Definir a chave de API do OpenRouter:",
					Command:     "caramel config set openrouter_key sk-or-v1-suachaveaqui...",
				},
			},
		},
		{
			Name:     "caramel guide",
			Short:    "Central de Ajuda Interativa e Didática do Caramel CLI",
			Category: CategorySystem,
			PedagogicalContext: `💡 QUANDO USAR:
Navegue pelo guia didático do Caramel CLI no terminal. Veja explicações detalhadas de cada comando,
dicas de uso pedagógico e exemplos práticos copiáveis.`,
			Syntax: "caramel guide [flags] ou caramel help -i",
			Flags: []FlagDoc{
				{Flag: "-i, --interactive", Description: "Abre a central de ajuda no modo TUI interativo com menu navegável."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Abrir o guia de ajuda interativo:",
					Command:     "caramel guide",
				},
			},
		},
		{
			Name:     "caramel version",
			Short:    "Exibe a versão atual do Caramel CLI",
			Category: CategorySystem,
			PedagogicalContext: `💡 QUANDO USAR:
Verifica a versão instalada do Caramel CLI, hash do commit de compilação e data da release.`,
			Syntax: "caramel version",
			Flags:  []FlagDoc{},
			Examples: []ExampleDoc{
				{
					Description: "Exibir informações de versão:",
					Command:     "caramel version",
				},
			},
		},
	}
}
