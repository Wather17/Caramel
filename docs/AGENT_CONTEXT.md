# Contexto do Caramel para agentes e skills

Este é o ponto de entrada canônico para qualquer agente ou skill que precise
entender rapidamente o Caramel. Ele apresenta o modelo mental do produto e
indica onde buscar detalhes. Não substitui o código nem os documentos
especializados.

## O que é o Caramel

O Caramel é uma CLI multiplataforma escrita em Go para criação de atividades,
tratamento de materiais pedagógicos e utilitários de produção. Ele organiza
fluxos que normalmente envolveriam vários scripts e ferramentas: ler e
reconstruir DOCX, gerar ou colorir imagens, montar materiais para impressão,
processar PDFs e consolidar rotinas de aula.

O nome oficial do executável é `caramel`. Após a instalação, `mel` é um alias
compatível e aceita a mesma árvore de comandos, flags, entradas e saídas.

## Modelo mental

Pense no Caramel como duas interfaces sobre os mesmos fluxos:

- **CLI**: comandos explícitos e scriptáveis, como `caramel image generate`,
  `caramel pdf split` e `caramel print 2up`.
- **Workspace visual**: a TUI aberta por `caramel workspace`, usada para
  importar, pesquisar, selecionar e encadear operações sobre materiais.

Os comandos de domínio ficam em `internal/cli/`, as regras de negócio ficam em
`internal/tools/` e os fluxos compartilhados ficam em `internal/workflow/`. A
workspace usa `internal/ui/` e `internal/vault/`.

O fluxo geral é:

```text
entrada (DOCX, imagem, PDF ou dados)
        -> ferramenta ou workflow do Caramel
        -> artefato de saída + diagnóstico/proveniência
```

## Fluxos principais

| Fluxo | Comandos ou entrada | Finalidade |
| --- | --- | --- |
| Documentos Word | `caramel docx images`, `docx merge`, `docx split` | Extrair imagens, juntar ou dividir DOCX preservando o que é suportado. |
| Imagens e IA | `caramel image generate`, `image colorize` | Gerar coleções pedagógicas ou colorir imagens e DOCX. |
| PDFs | `caramel pdf create`, `merge`, `split`, `pages render`, `images extract` | Criar, combinar, dividir, renderizar páginas ou extrair imagens. |
| Impressão | `caramel print cards`, `caramel print 2up` | Montar fichas A4 ou duas atividades por folha. |
| Rotinas | `caramel routine consolidate` | Consolidar rotinas semanais e classificar campos de experiência. |
| Acervo visual | `caramel workspace` | Importar, buscar e encadear operações sobre o vault global. |

Para descobrir a sintaxe atual, prefira a própria CLI:

```bash
caramel guide
caramel guide <termo>
caramel <comando> --help
```

## Invariantes importantes

- A árvore de comandos e os textos de ajuda são derivados da definição Cobra
  no código. `docs/COMMANDS.md` explica organização e compatibilidade, mas não
  deve ser tratado como uma lista manual completa de comandos.
- A workspace usa um vault global, normalmente no diretório de dados do
  usuário. `CARAMEL_VAULT_DIR` pode redirecioná-lo.
- A biblioteca visível é configurada separadamente por `CARAMEL_LIBRARY_DIR` ou
  `caramel vault init`; fontes vinculadas são indexadas sem serem movidas ou apagadas.
- Materiais do vault são deduplicados por hash, armazenados uma vez e
  indexados; coleções referenciam materiais e execuções registram
  proveniência, saídas e derivações.
- O vault não substitui automaticamente diretórios de projetos. DOCX, PDFs e
  outros arquivos de trabalho ainda podem precisar permanecer em seus projetos
  e destinos explícitos.
- Operações que usam serviços externos devem respeitar custos, retries,
  rate limits, fallback, segurança de URLs e persistência de diagnóstico.
- Mudanças de comportamento público devem preservar compatibilidade quando
  possível e atualizar a documentação especializada correspondente.

## Onde buscar detalhes

| Preciso entender... | Leia primeiro |
| --- | --- |
| Arquitetura, pacotes e fluxo interno | [`ARCHITECTURE.md`](ARCHITECTURE.md) |
| Sintaxe, grupos, aliases e exemplos de comandos | [`COMMANDS.md`](COMMANDS.md) e `caramel --help` |
| Compilação, instalação e execução local | [`GETTING_STARTED.md`](GETTING_STARTED.md) |
| Como criar um novo comando | [`CONTRIBUTING_COMMANDS.md`](CONTRIBUTING_COMMANDS.md) |
| Saída textual, JSON, quiet e verbose | [`OUTPUT_UX.md`](OUTPUT_UX.md) e [`OUTPUT_UX_MATRIX.md`](OUTPUT_UX_MATRIX.md) |
| IA, integrações, retries e limites | [`API_INTEGRATIONS.md`](API_INTEGRATIONS.md) |
| Workspace, vault e proveniência | `ARCHITECTURE.md` e `COMMANDS.md` |
| Releases e versionamento | [`RELEASES.md`](RELEASES.md) |

## Orientação para skills dependentes do Caramel

Uma skill que use o Caramel deve apontar para este arquivo como seu contexto
inicial canônico e, em seguida, indicar apenas os documentos especializados
necessários para aquela tarefa. Evite copiar este conteúdo para a skill: a
duplicação cria versões divergentes.

Quando a tarefa depender de comportamento real, confirme a implementação no
código e valide com a CLI ou os testes. Este documento é um mapa de navegação,
não uma autorização para presumir que uma capacidade existe.
