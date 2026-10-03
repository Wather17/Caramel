# Matriz de saída por comando

Esta matriz é a referência rápida para revisar a saída dos comandos públicos.

| Área/comando | Categoria e formato | Sucesso e artefato | Vazio, aviso ou falha parcial |
| --- | --- | --- | --- |
| `docx images list` | Sem output | Quantidade e lista compacta de imagens. | Resultado vazio explícito; falhas de leitura identificadas. |
| `docx images extract` / `docx extract` | `docx`, pacote | Quantidade e pasta principal. | Sem imagens não publica pasta; parcial retorna `warning`. |
| `docx split` | `docx`, pacote | Pasta com todas as partes. | Falha descarta staging. |
| `docx merge` | `docx`, único | DOCX final; destino pode ser omitido no modo biblioteca. | Fora do modo biblioteca, destino continua obrigatório. |
| `image colorize` | `imagens`; único para uma imagem, pacote para pasta/DOCX | Arquivo ou pasta principal. | Artefatos válidos são preservados com `warning`. |
| `image generate` | `imagens`, pacote sempre | Pasta da coleção, mesmo com um item. | Sem artefatos não publica pasta. |
| `print 2up` / `print cards` | `impressao`, único | PDF ou HTML final. | Falha descarta staging. |
| `pdf create` / `pdf merge` | `pdf`, único | PDF final. | Merge sem destino só funciona no modo biblioteca. |
| `pdf split` / `pdf pages render` / `pdf images extract` | `pdf`, pacote | Pasta principal; JSON contém arquivos. | Sem artefatos não publica pasta; parcial retorna `warning`. |
| `routine consolidate` | `rotinas`, único | Relatório DOCX consolidado. | `warning` para consolidação parcial. |
| `config show` | — | Configuração segura e status. | Nunca expõe segredos; informa valores ausentes. |
| `config models list/select` | — | Modelos ou modelo selecionado. | Aviso quando nenhum modelo estiver disponível. |
| `install` | — | Confirmação curta da instalação. | Falha com próximo passo sugerido. |
| `version` | — | Versão em uma linha. | — |
| `guide` | — | Guia solicitado sem ruído adicional. | Comando ou tópico desconhecido com orientação. |
| `workspace` | — | Resumo da operação e artefatos. | Estados explícitos para aviso, cancelamento e falha. |

## Regras de revisão manual

- O modo padrão deve caber em uma leitura rápida e indicar o próximo passo quando necessário.
- A ausência de itens deve ser distinguida de uma execução bem-sucedida com itens.
- Avisos, itens ignorados e falhas parciais devem aparecer no resumo final.
- Diagnósticos longos e conteúdo bruto só devem aparecer com `--verbose`.
- `--quiet` não pode esconder erros; `--json` deve produzir um único resultado parseável em `stdout`.
