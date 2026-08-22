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
	Keywords           []string        // Palavras-chave pedagógicas para busca rápida (ex: "figma", "papel", "2up")
}

// GetAllCommandDocs retorna a central de documentação didática de todos os comandos do Caramel
func GetAllCommandDocs() []CommandHelpDoc {
	return []CommandHelpDoc{
		{
			Name:     "caramel 2up",
			Short:    "Monta PDF A4 Paisagem com 2 atividades/páginas lado a lado (economiza papel)",
			Category: CategoryDocx,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize este comando para preparar avaliações, atividades ou apostilas em formato 2 por folha (2-up),
imprimindo duas páginas A5 lado a lado em uma única folha A4 em orientação Paisagem.

O Caramel irá:
 1. Ler uma imagem isolada ou uma pasta inteira contendo imagens de atividades (PNG, JPG, WEBP);
 2. Ordenar os arquivos numericamente (ex: img1, img2, img10);
 3. Redimensionar e comprimir imagens pesadas do Figma (exportadas em 4x) em memória a 300 DPI (PDFs de ~500KB);
 4. Identificar imagens horizontais (landscape) e aplicar auto-rotação 90° inteligente se ganhar área útil;
 5. Inserir a linha tracejada central orientativa para corte.`,
			Syntax: "caramel 2up <imagem_ou_pasta> [flags]",
			Keywords: []string{
				"2up", "print", "layout", "figma", "papel", "economizar", "duplicar",
				"a5", "a4", "dpi", "otimizar", "comprimir", "rotacionar", "corte", "impressao",
			},
			Flags: []FlagDoc{
				{Flag: "-r, --auto-rotate", Description: "Rotaciona automaticamente imagens landscape em 90° se a área útil aumentar (padrão: true)."},
				{Flag: "-t, --rotate-threshold", Description: "Porcentagem mínima de ganho de área útil para disparar a rotação (padrão: 15.0%)."},
				{Flag: "-f, --fit", Description: "Modo de encaixe no slot: 'contain' (sem cortes na atividade) ou 'cover' (preenchimento total)."},
				{Flag: "-O, --optimize", Description: "Otimiza e comprime imagens pesadas (ex: Figma 4x) em memória a 300 DPI para gerar PDFs leve de ~500KB (padrão: true)."},
				{Flag: "--max-dpi", Description: "Resolução máxima em DPI para renderização de imagens no PDF (padrão: 300)."},
				{Flag: "-q, --quality", Description: "Qualidade de compressão JPEG de 1 a 100 (padrão: 85)."},
				{Flag: "-l, --cut-line", Description: "Desenha a linha tracejada central de corte (padrão: true)."},
				{Flag: "-d, --duplicate", Description: "Duplica imagens ímpares ou isoladas no segundo slot da folha (padrão: true)."},
				{Flag: "-m, --margin", Description: "Margem externa da página em milímetros (padrão: 5mm)."},
				{Flag: "-o, --output", Description: "Caminho do arquivo PDF ou pasta de destino personalizada."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Gerar PDF 2-up economizando papel a partir de uma pasta de atividades exportadas do Figma:",
					Command:     "caramel 2up ./atividades_figma",
				},
				{
					Description: "Gerar 2-up de uma prova única em A5 duplicando ela no segundo slot para corte:",
					Command:     "caramel 2up prova_historia.png",
				},
				{
					Description: "Gerar 2-up forçando preenchimento total do slot (cover) sem margem em branco:",
					Command:     "caramel 2up ./fichas_estudo -f cover",
				},
			},
		},
		{
			Name:     "caramel cards",
			Short:    "Gera layout HTML A4 de fichas pedagógicas/flashcards pronto para impressão e corte",
			Category: CategoryDocx,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize este comando para diagramar coleções de imagens (PNG, JPG, WEBP) em folhas A4
com proporção 1:1, legendas com o nome do objeto centralizado embaixo e linhas tracejadas de tesoura ✂️.

O Caramel irá:
 1. Ler todas as imagens de uma pasta e extrair os nomes higienizados;
 2. Distribuir os cartões em grade proporcional (ex: 2x3 = 6 fichas por folha A4);
 3. Gerar um arquivo HTML independente com Tailwind CSS embutido;
 4. Permitir impressão direta (Ctrl+P) ou salvamento em PDF em qualquer navegador.`,
			Syntax: "caramel cards <pasta_ou_imagem> [flags]",
			Keywords: []string{
				"cards", "flashcards", "fichas", "impressao", "a4", "recorte", "tesoura", "alfabetizacao", "vocabulario", "jogo da memoria",
			},
			Flags: []FlagDoc{
				{Flag: "-c, --cols", Description: "Número de colunas por folha A4 (padrão: 2)."},
				{Flag: "-r, --rows", Description: "Número de linhas por folha A4 (padrão: 3)."},
				{Flag: "-t, --title", Description: "Título exibido no topo de cada folha."},
				{Flag: "-l, --cut-lines", Description: "Exibe linhas tracejadas para corte de tesoura (padrão: true)."},
				{Flag: "-u, --uppercase", Description: "Exibe o nome das fichas em caixa alta para alfabetização (padrão: true)."},
				{Flag: "-o, --output", Description: "Caminho do arquivo HTML de saída."},
				{Flag: "-e, --embed", Description: "Embute as imagens em Base64 (arquivo 100% autossuficiente)."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Gerar fichas A4 de uma pasta de imagens:",
					Command:     "caramel cards ./minhas_figuras/",
				},
				{
					Description: "Gerar grade 3x3 (9 cartões por folha) com título customizado:",
					Command:     "caramel cards ./animais/ -c 3 -r 3 -t \"Animais da Savana\"",
				},
			},
		},
		{
			Name:     "caramel generate",
			Short:    "Gera ilustrações e coleções de objetos pedagógicos em lote com IA",
			Category: CategoryDocx,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize este comando para criar coleções visuais inteiras de imagens e ilustrações (ex: 5, 10, 30+ itens)
a partir de palavras simples (frutas, animais, legumes, profissões, ações) ou de um tema descritivo.

O Caramel irá:
 1. Usar IA para sintetizar e padronizar prompts em inglês com fundo branco e estilo unificado;
 2. Gerar as imagens com concorrência adaptativa (worker pool inteligente sem bloqueios de rate limit);
 3. Exibir miniaturas ANSI em tempo real no terminal;
 4. Opcionalmente compilar tudo em um PDF 2-up pronto para impressão (--2up).`,
			Syntax: "caramel generate [itens...] ou caramel image generate [flags]",
			Keywords: []string{
				"generate", "gerar", "harness", "lote", "batch", "imagens", "ilustracoes", "clipart", "3d", "vector", "ia", "flash-image",
			},
			Flags: []FlagDoc{
				{Flag: "-i, --items", Description: "Lista de itens separados por vírgula (ex: 'maçã, banana, melancia')."},
				{Flag: "-f, --file", Description: "Arquivo .txt contendo os itens (um por linha)."},
				{Flag: "-t, --theme", Description: "Tema descritivo para a IA escolher os itens automaticamente (ex: 'animais da fazenda')."},
				{Flag: "-n, --count", Description: "Quantidade de itens ao usar com --theme (padrão: 10)."},
				{Flag: "-s, --style", Description: "Estilo visual: clipart, vector, 3d-cute, coloring, realistic (padrão: clipart)."},
				{Flag: "--2up", Description: "Compila automaticamente todas as imagens geradas em um PDF 2-up A4."},
				{Flag: "--preview", Description: "Exibe preview ANSI no terminal durante a geração (padrão: true)."},
				{Flag: "-o, --output", Description: "Diretório onde as imagens serão salvas."},
				{Flag: "-w, --workers", Description: "Número de workers concorrentes (padrão: adaptativo)."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Gerar imagens de frutas tropicais em estilo 3D fofo:",
					Command:     "caramel generate --items \"abacaxi, manga, maracujá, caju\" -s 3d-cute",
				},
				{
					Description: "Gerar 10 animais da fazenda e compilar em PDF 2-up:",
					Command:     "caramel generate --theme \"animais da fazenda\" -n 10 --2up",
				},
				{
					Description: "Gerar desenhos para colorir a partir de um arquivo:",
					Command:     "caramel generate -f ./itens.txt -s coloring",
				},
			},
		},
		{
			Name:     "caramel colorize",
			Short:    "Colora ilustrações/fotos ou processa e reconstrói documentos .docx com IA",
			Category: CategoryDocx,
			PedagogicalContext: `💡 QUANDO USAR:
Utilize este comando para colorir imagens individuais (PNG, JPG, WEBP), pastas de ilustrações
ou processar documentos .docx diretamente com a IA do OpenRouter.

Ao receber um arquivo .docx:
 1. Executa o pipeline automatizado de extração, coloração e redimensionamento;
 2. Reconstrói um novo arquivo Word colorido ('<nome> colorida.docx');
 3. Suporta a flag -i para seleção interativa com preview ANSI no terminal.

Antes de colorir, uma triagem de economia analisa cada imagem localmente (saturação) e com um
modelo de visão gratuito: imagens já coloridas ou contendo apenas texto/tabelas são puladas
automaticamente, evitando gastos desnecessários. Use --no-triage para desativar.`,
			Syntax: "caramel colorize <imagem | pasta | arquivo.docx> [flags]",
			Keywords: []string{
				"colorize", "color", "colorir", "ia", "openrouter", "docx", "imagem", "png", "jpg", "foto", "desenho",
			},
			Flags: []FlagDoc{
				{Flag: "-i, --interactive", Description: "Habilita seleção interativa com preview ANSI TrueColor no terminal."},
				{Flag: "-a, --all", Description: "Colora todas as imagens encontradas sem abrir formulário de seleção."},
				{Flag: "-s, --min-size", Description: "Tamanho mínimo da imagem ao processar .docx (ex: '20KB', '50KB', '0' para todas)."},
				{Flag: "-m, --model", Description: "Modelo de IA multimodal do OpenRouter (padrão: google/gemini-3.1-flash-image-preview)."},
				{Flag: "--triage-model", Description: "Modelo de visão da triagem de economia (padrão: qwen/qwen3.7-flash)."},
				{Flag: "--no-triage", Description: "Desativa a triagem e colora todas as imagens selecionadas diretamente."},
				{Flag: "-o, --output", Description: "Diretório de destino para salvar imagens/documento coloridos."},
				{Flag: "-v, --verbose", Description: "Exibe detalhes de depuração e resposta da API."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Colorir uma ilustração isolada:",
					Command:     "caramel colorize desenho.png",
				},
				{
					Description: "Processar e reconstruir uma prova .docx com imagens coloridas:",
					Command:     "caramel colorize avaliacao.docx",
				},
				{
					Description: "Colorir imagens de um .docx com seleção interativa no terminal:",
					Command:     "caramel colorize atividade.docx -i",
				},
			},
		},
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
			Keywords: []string{
				"process", "pipeline", "colorir", "ia", "openrouter", "docx", "word", "prova", "apostila", "p&b",
			},
			Flags: []FlagDoc{
				{Flag: "-i, --interactive", Description: "Abre o menu interativo com checkboxes para você escolher visualmente quais imagens deseja colorir."},
				{Flag: "-s, --min-size", Description: "Tamanho mínimo das imagens a serem processadas (ex: '20KB', '50KB', '0' para todas)."},
				{Flag: "-m, --model", Description: "Modelo de IA do OpenRouter para coloração (padrão: google/gemini-3.1-flash-image-preview)."},
				{Flag: "--triage-model", Description: "Modelo de visão da triagem de economia (padrão: qwen/qwen3.7-flash)."},
				{Flag: "--no-triage", Description: "Desativa a triagem e colora todas as imagens elegíveis diretamente."},
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
			Keywords: []string{
				"extract", "extrair", "figuras", "imagens", "docx", "word", "listar", "salvar",
			},
			Flags: []FlagDoc{
				{Flag: "-l, --list", Description: "Apenas inspeciona e exibe a lista de imagens do arquivo sem salvá-las."},
				{Flag: "-i, --interactive", Description: "Abre o menu interativo para você selecionar apenas as figuras que deseja salvar."},
				{Flag: "-c, --colorize", Description: "Ativa a IA para colorir as imagens extraídas."},
				{Flag: "-s, --min-size", Description: "Define o tamanho mínimo para filtrar figuras (padrão: '0' — todas)."},
				{Flag: "--triage-model", Description: "Modelo de visão da triagem de economia (padrão: qwen/qwen3.7-flash)."},
				{Flag: "--no-triage", Description: "Desativa a triagem e colora todas as imagens elegíveis diretamente."},
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
			Keywords: []string{
				"config", "setup", "chave", "api", "openrouter", "chave-api", "configurar",
			},
			Flags: []FlagDoc{},
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
			Keywords: []string{
				"config", "show", "status", "env", "chave", "onde",
			},
			Flags: []FlagDoc{},
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
			Keywords: []string{
				"config", "set", "chave", "definir", "salvar",
			},
			Flags: []FlagDoc{},
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
dicas de uso pedagógico, busca por palavra-chave e exemplos práticos copiáveis.`,
			Syntax: "caramel guide [termo_de_busca] ou caramel help [termo_de_busca]",
			Keywords: []string{
				"guide", "help", "ajuda", "duvida", "comando", "exemplo", "como-usar", "tui",
			},
			Flags: []FlagDoc{
				{Flag: "-i, --interactive", Description: "Abre a central de ajuda no modo TUI interativo com menu navegável."},
			},
			Examples: []ExampleDoc{
				{
					Description: "Abrir o guia de ajuda interativo:",
					Command:     "caramel guide",
				},
				{
					Description: "Pesquisar guia sobre imagens do Figma:",
					Command:     "caramel guide figma",
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
			Keywords: []string{
				"version", "versao", "atualizacao", "release",
			},
			Flags: []FlagDoc{},
			Examples: []ExampleDoc{
				{
					Description: "Exibir informações de versão:",
					Command:     "caramel version",
				},
			},
		},
	}
}
