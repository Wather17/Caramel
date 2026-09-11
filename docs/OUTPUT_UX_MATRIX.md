# Matriz de saída por comando

Esta matriz é a referência rápida para revisar a saída dos comandos públicos.

| Área/comando | Sucesso e artefato | Operação longa ou `--verbose` | Vazio, aviso ou falha parcial |
| --- | --- | --- | --- |
| `docx images list` | Quantidade e lista compacta de imagens. | Metadados por imagem em `stderr`. | Resultado vazio explícito; falhas de leitura identificadas. |
| `docx images extract` | Quantidade e diretório de saída. | Progresso e falhas por item em `stderr`. | `warning` quando apenas parte dos itens falhar. |
| `image colorize` | Arquivo de saída e resumo da etapa. | Etapas e diagnósticos do pipeline. | `failed` com causa curta; sem conteúdo bruto por padrão. |
| `image generate` | Arquivo(s) gerado(s) e parâmetros essenciais. | Progresso e resposta detalhada do provedor. | `failed` com causa acionável e artefatos parciais, se houver. |
| `print 2up` | Arquivo PDF gerado e contagem de páginas. | Detalhes de layout e validação. | Aviso para entradas vazias ou páginas ignoradas. |
| `print cards` | Arquivo de cartões e contagem. | Detalhes de agrupamento e itens. | Resumo de itens ignorados e falhas parciais. |
| `routine consolidate` | Resumo agregado da consolidação. | Eventos e diagnóstico por etapa. | `warning` para fontes ausentes ou consolidação parcial. |
| `config show` | Configuração segura e status. | Diagnóstico de resolução de configuração. | Nunca expõe segredos; informa valores ausentes. |
| `config models list/select` | Modelos ou modelo selecionado. | Detalhes de descoberta e validação. | Aviso quando nenhum modelo estiver disponível. |
| `install` | Confirmação curta da instalação. | Comandos e passos executados. | Falha com próximo passo sugerido. |
| `version` | Versão em uma linha. | Informações adicionais de build. | — |
| `guide` | Guia solicitado sem ruído adicional. | Conteúdo expandido apenas quando solicitado. | Comando ou tópico desconhecido com orientação. |
| `workspace` | Resumo da operação e artefatos. | Eventos detalhados por etapa. | Estados explícitos para aviso, cancelamento e falha. |

## Regras de revisão manual

- O modo padrão deve caber em uma leitura rápida e indicar o próximo passo quando necessário.
- A ausência de itens deve ser distinguida de uma execução bem-sucedida com itens.
- Avisos, itens ignorados e falhas parciais devem aparecer no resumo final.
- Diagnósticos longos e conteúdo bruto só devem aparecer com `--verbose`.
- `--quiet` não pode esconder erros; `--json` deve produzir um único resultado parseável em `stdout`.
