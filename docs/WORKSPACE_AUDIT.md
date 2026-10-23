# Auditoria do workspace universal

Data da auditoria: 2026-10-22
Issue: #101
Escopo: diagnóstico somente-leitura de `caramel workspace`, integrações do vault,
fluxos legados, picker DOCX e abertura de arquivos.

## Conclusão executiva

`caramel workspace` já entra no workspace do vault em produção. O caminho observado é:

```text
caramel workspace
  -> internal/cli/workspace.go: workspaceCmd.RunE
  -> internal/ui.RunWorkspace
  -> internal/ui.RunVaultWorkspace
  -> NewVaultWorkspaceModel
  -> vault.Open + MigrateLegacyProjects + Vault.Search
```

O `WorkspaceModel` de `internal/ui/workspace.go` é um modelo antigo de projetos. Ele ainda
compila e contém helpers compartilhados, mas não é alcançado pelo ponto de entrada atual:
`RunWorkspace` é um shim que chama `RunVaultWorkspace`. Isso é evidência de redundância no
caminho de produção, não prova de que usuários externos não dependam de símbolos ou do formato
legado. A remoção deve ser uma etapa própria, precedida pela extração dos helpers e por gates de
migração.

Nenhum código, teste, dado do vault, output ou configuração foi alterado nesta auditoria.

## Método e evidências reproduzíveis

Foram combinadas referências estáticas, testes existentes, histórico e uma execução read-only do
comando. Os comandos abaixo foram executados na raiz do repositório em `main` antes deste
relatório:

| Comando | Resultado | O que comprova |
| --- | --- | --- |
| `rg -n 'workflow\\.(ImageService\\|VaultImageService)\\|ImageService\\{' internal --glob '*.go'` | O serviço ativo aparece em `vault_workspace.go`; o legado aparece no modelo antigo e em testes. | Separação entre os dois serviços. |
| `rg -n 'NewWorkspaceModel\\|WorkspaceModel' internal --glob '*.go'` | As referências do modelo antigo ficam no próprio arquivo; o modelo ativo é `VaultWorkspaceModel`. | Não há chamada interna observada para `NewWorkspaceModel`. |
| `rg -n 'NewVaultWorkspaceModel\\|RunVaultWorkspace\\|RunWorkspace' internal --glob '*.go'` | `cli/workspace.go` chama `RunWorkspace`; o shim chama `RunVaultWorkspace`. | Ponto de entrada efetivo. |
| `go test ./internal/ui ./internal/cli ./internal/workspace ./internal/vault ./internal/platform` | Passou. | Cobertura relacionada permanece verde. |
| `go run ./cmd/caramel workspace --help` | Passou e exibiu a ajuda de `caramel workspace`. | Registro e execução do comando sem abrir a TUI. |
| `go test ./...` | Passou. | Regressão geral. |
| `go vet ./...` | Passou. | Validação estática. |
| `CGO_ENABLED=0 go build ./...` | Passou. | Todos os pacotes continuam compiláveis. |
| `git diff --check` | Passou. | O artefato não contém erro de whitespace. |
| `git log --oneline --follow -- internal/ui/workspace.go` | Mostra a evolução do modelo antigo e a introdução do vault workspace. | Evidência histórica de coexistência e migração em andamento. |

O smoke completo da TUI não foi executado porque esta sessão não fornece um TTY interativo nem
um desktop para observar `open`/`reveal`. Essa limitação foi registrada, e a ausência de execução
gráfica não foi usada como evidência de código morto.

## Inventário do caminho atual

### Entrada, telas e estados

O ponto de entrada CLI é `internal/cli/workspace.go`. O modelo efetivamente executado é
`internal/ui/VaultWorkspaceModel`, com estes estados alcançáveis:

| Estado | Entrada observada | Responsabilidade |
| --- | --- | --- |
| `vaultInbox` | inicial, `r`, busca | Lista materiais do vault e mantém a seleção por ID. |
| `vaultImport` | `i` ou `import` | Importa arquivo/pasta via `Vault.ImportPaths`. |
| `vaultGenerate` | `g` ou `generate` | Recebe itens e prepara geração. |
| `vaultConfirm` | `c`, `f`, `p`, `g` | Confirma a operação antes de executá-la. |
| `vaultRunning` | confirmação | Executa operação assíncrona e aceita cancelamento. |
| `vaultConsole` | `:` | Console restrito com histórico e retorno à inbox. |

Atalhos observados na inbox: `/` busca, `j/k` ou setas navegam, `space` alterna seleção,
`a` seleciona todos, `n` limpa seleção, `i` importa, `g` gera, `c` colore, `f` cria cards,
`p` cria 2-up, `r` recarrega e `q` sai. `esc` cancela entradas/operações conforme o estado.

O console de `internal/ui/console.go` contém os comandos `help`, `search`, `import`, `generate`,
`colorize`, `cards`, `2up`, `clear`, `back` e `quit`. Os aliases observados são `find`, `i`, `g`,
`c`, `f`, `p` e `exit`. O console opera sobre o mesmo vault e a mesma seleção da inbox; não é
uma segunda persistência.

### Componentes e dependências

| Componente | Referências/testes observados | Classificação | Confiança | Justificativa |
| --- | --- | --- | --- | --- |
| `internal/cli/workspace.go` | `workspaceCmd` registrado no root; `root_test.go` cobre registro. | **Manter** | Alta | É a entrada pública do comando. |
| `internal/ui/vault_workspace.go` | `RunVaultWorkspace`, `NewVaultWorkspaceModel`, estados e fluxo de operações. | **Manter** | Alta | É o caminho atual da TUI e integra o vault global. |
| `internal/ui/console.go` | Métodos de `VaultWorkspaceModel`; `console_test.go` cobre parsing/ajuda. | **Manter, depois adaptar** | Alta | É uma interface ativa, mas seus comandos devem convergir para ações reutilizáveis do hub. |
| `internal/ui/workspace.go` | `WorkspaceModel` e `NewWorkspaceModel` só aparecem no próprio arquivo; `RunWorkspace` é shim. | **Remover em etapa própria** | Média-alta | Redundante no call path atual, mas contém helpers usados pela TUI ativa e não há prova de que símbolos não sejam usados fora do repositório. |
| `internal/workflow/vault_images.go` | Chamado por `VaultWorkspaceModel`; testes em `context_test.go`. | **Manter** | Alta | É o serviço de geração, coloração, cards e 2-up baseado em materiais/resultados do vault. |
| `internal/workflow/images.go` | Chamado pelo modelo antigo; coberto por `images_test.go` e parte de `context_test.go`. | **Remover depois da migração** | Média-alta | Serviço legado de projetos, ainda coberto e necessário para preservar compatibilidade durante a migração. |
| `internal/workspace/` | Usado pelo serviço legado, testes e `vault/migration.go`. | **Preservar; adaptar** | Alta | O leitor/migrador de manifestos JSON antigos ainda é caminho ativo. Não deve ser apagado junto com a TUI antiga. |
| `internal/vault.Vault` e `Material` | `Search`, `ImportPaths`, runs, coleções e derivação usados pela TUI; testes em `vault_test.go`/`library_test.go`. | **Manter** | Alta | É a fonte de verdade do workspace universal. |
| `vault.SearchIndexedFiles`, `SyncIfStale`, `TouchIndexedFile` | Implementados em `internal/vault/library.go`; usados pelo fluxo de completion/indexação. | **Manter; integrar** | Alta | Complementam a descoberta de arquivos externos sem duplicá-los no vault. |
| `internal/vault/migration.go` | Chamado na criação de `VaultWorkspaceModel`. | **Manter até a migração final** | Alta | Compatibilidade com projetos antigos é requisito explícito. |
| `internal/platform/opener.go` | Usado por `docx_merge.go` e `result_open.go`; testes cobrem seleção de comandos. | **Manter; reutilizar** | Alta | Centraliza `open`/`reveal` por plataforma. |
| picker DOCX (#100) | `internal/cli/docx_merge.go` usa `platform.Opener` para revelar o resultado. | **Preservar como referência** | Alta | Deve fornecer o contrato de seleção por tipo/ação sem ser reimplementado pelo hub. |
| docs e testes relacionados | `docs/ARCHITECTURE.md`, `docs/COMMANDS.md`, `docs/GETTING_STARTED.md`, testes por pacote. | **Manter e ampliar** | Alta | Devem acompanhar cada etapa, principalmente migração e cobertura da TUI. |

“Não observado” foi usado somente para ausência de referência no repositório; não foi convertido
em afirmação de código morto.

## Comparação de capacidades

| Capacidade | Workspace atual | Vault/completion | Picker DOCX/opener | Destino recomendado |
| --- | --- | --- | --- | --- |
| Descoberta | Inbox de `Material` e busca textual do vault. | `Material` persistente e `IndexedFile` para arquivos externos/indexados. | Picker conhece um fluxo específico de documentos. | Read model federado que preserve a origem de cada item. |
| Busca | `Vault.Search` na inbox e `search` no console. | `SearchIndexedFiles` cobre candidatos fora do vault; completion indexada é outra superfície. | Busca deve ser delegada ao picker/ação quando o tipo exigir. | Unificar consulta e filtros sem copiar automaticamente arquivos externos. |
| Seleção | IDs de `Material`, mantidos em memória durante a sessão. | Candidatos indexados ainda precisam de identidade/ação próprias. | Picker seleciona arquivos adequados ao `docx merge`. | Modelo de seleção por tipo e ação, com origem e confirmação explícitas. |
| Importação | `ImportPaths` copia para o vault e marca seleção. | Vault é o destino persistente; indexação pode observar sem importar. | DOCX deve preservar o contrato de entrada do picker. | Separar `usar`, `importar` e `revelar`; evitar cópia implícita. |
| Preview | A TUI mostra metadados; outputs são acompanhados pelo fluxo de execução. | O vault conhece `ObjectPath`, materiais e resultados. | `platform.Opener` revela resultado no gerenciador de arquivos. | Adicionar preview/abrir/revelar por tipo sem duplicar lógica de plataforma. |
| Execução | Geração, coloração, cards e 2-up via `VaultImageService`; runs e coleções são persistidos. | Vault registra runs, outputs e derivações. | Picker DOCX inicia a operação específica. | Reutilizar serviços de domínio; hub apenas seleciona, confirma e acompanha. |
| Histórico | O modelo ativo mantém eventos da execução e o vault persiste runs. | `StartRun`/`FinishRunWithAttempts` fornecem base para histórico. | Opener não persiste histórico. | Expor histórico de runs/outputs como uma capacidade do hub. |

O diferencial atual do workspace é a composição TUI: inbox, seleção persistente da sessão,
confirmação, progresso e console sobre serviços do vault. O que não deve ser duplicado é a
persistência, a indexação, a execução dos fluxos e a lógica de abertura de arquivos.

## Saldo de escopo

Os tamanhos medidos no estado auditado são:

| Área | Tamanho aproximado | Leitura |
| --- | ---: | --- |
| `internal/ui/workspace.go` | 578 linhas | Candidato principal à remoção futura; mistura modelo legado, telas e helpers compartilhados. |
| `internal/ui/vault_workspace.go` | 494 linhas | Caminho ativo; deve ser dividido apenas quando houver uma abstração de seleção/ação testada. |
| `internal/ui/console.go` | 268 linhas | Caminho ativo; pode virar adaptador fino sobre comandos reutilizáveis. |
| `internal/workspace/workspace.go` | 490 linhas | Compatibilidade de projetos; não é candidato imediato a remoção. |
| `internal/workflow/images.go` | 398 linhas | Serviço legado acoplado ao modelo antigo. |
| `internal/workflow/vault_images.go` | 332 linhas | Serviço ativo a preservar. |
| `internal/platform/opener.go` | 96 linhas | Adaptador multiplataforma pequeno e reutilizável. |

Saldo provável, sem executar ainda: retirar uma TUI legada de aproximadamente 578 linhas e um
serviço legado de aproximadamente 398 linhas, depois de extrair helpers e concluir a migração.
Isso não autoriza apagar os arquivos agora. O trabalho novo necessário é um read model federado,
um picker/seleção orientado a tipo e ação, histórico de runs/outputs e testes de comportamento da
TUI; os tamanhos desses componentes não são estimados sem uma decisão de produto.

## Riscos e gates

- **Migração:** projetos JSON antigos podem ser a única fonte de materiais de alguns usuários.
  `MigrateLegacyProjects` deve continuar idempotente e coberto antes de remover escritores/leitores.
- **Persistência da seleção:** hoje a seleção usa IDs de `Material` em memória; itens indexados
  precisam de uma identidade estável sem serem copiados automaticamente.
- **Outputs e histórico:** geração, coloração, cards e 2-up registram resultados, tentativas,
  coleções e derivações. A remoção do serviço antigo não pode perder outputs nem mudar o status
  de cancelamento/erro.
- **TTY/TUI:** estados de confirmação, cancelamento e retorno do console são comportamento público
  observável. A ausência de TTY nesta auditoria não é um passe para removê-los.
- **Plataformas:** `platform.Opener` encapsula Linux, macOS e Windows. O hub não deve chamar
  `xdg-open`, `open` ou `explorer` diretamente.
- **Cobertura:** há testes de domínio e parsing, mas não há um smoke automatizado de toda a TUI;
  cada etapa de remoção deve adicionar testes de transição/ação antes de retirar o equivalente legado.
- **Compatibilidade externa:** ausência de referências em `rg` cobre o repositório, não scripts,
  automações ou usuários que importem pacotes internos.

## Roadmap atômico recomendado

1. **Extrair helpers compartilhados da TUI.** Mover `splitItems`, `operationLabel`,
   `configuredModel` e demais helpers realmente usados para um pacote/símbolo neutro; adicionar
   testes de contrato. Dependência: nenhuma. Gate: testes de `internal/ui` verdes e diff sem
   mudança de comportamento.
2. **Isolar compatibilidade de projetos legados.** Separar o leitor/migrador de
   `internal/workspace` das APIs escritoras e documentar o formato suportado. Dependência: etapa 1.
   Gate: testes de migração, idempotência e preservação de caminhos.
3. **Criar o read model federado.** Unificar `Material`, `IndexedFile` e outputs com origem,
   tipo, ações permitidas e política explícita de importação. Dependência: etapa 2. Gate: nenhum
   arquivo externo é copiado, movido ou removido por uma consulta.
4. **Extrair seleção por tipo/ação e preservar o picker DOCX.** Reutilizar o contrato final do
   #100 para seleção de documentos, imagens e resultados, sem transformar a issue em um picker
   universal monolítico. Dependência: etapa 3 e #100. Gate: `docx merge` e seus caminhos de erro
   permanecem verdes.
5. **Integrar histórico, outputs e `open/reveal`.** Expor runs/resultados no hub e delegar
   abertura a `platform.Opener`. Dependência: etapas 3–4. Gate: caminhos Linux/macOS/Windows
   continuam cobertos por testes do adaptador.
6. **Remover o modelo e o serviço legados.** Apagar `WorkspaceModel` e `ImageService` somente
   quando nenhum caminho de produção/teste necessário depender deles e os gates de migração,
   execução e TUI estiverem verdes. Dependência: etapas 1–5. Gate: `rg` sem referências não
   justificadas, `go test ./...`, `go vet ./...` e smoke em TTY disponível.

Não foram abertas issues filhas nesta auditoria: o diagnóstico fornece a ordem e as dependências,
mas cada mudança futura precisa ser aprovada como issue atômica própria antes da implementação.

## Decisões ainda de produto

Estas decisões não bloqueiam o diagnóstico factual:

- quais tipos entram na primeira versão do read model federado;
- se a seleção de arquivos externos oferece somente `reveal`, `import` ou também execução direta;
- quanto do console deve permanecer na TUI quando comandos de CLI equivalentes existirem;
- qual retenção e apresentação de histórico de runs/outputs o hub deve oferecer.

Recomendação: aprovar primeiro as etapas 1 e 2, porque reduzem o risco de uma remoção prematura;
depois implementar o read model e a seleção orientada a ações antes de qualquer poda.
