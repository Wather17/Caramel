# Contrato de saída e feedback do Caramel

Este documento define como os comandos públicos do Caramel comunicam progresso, resultado e falhas. O objetivo é manter a saída humana curta e útil, deixando detalhes operacionais disponíveis sob demanda.

## Princípios

- A saída normal responde rapidamente: o que aconteceu, qual foi o resultado e onde estão os artefatos.
- Detalhes por item, diagnósticos e conteúdo bruto ficam em `--verbose`.
- Operações com vários itens exibem um resumo final único, incluindo contagens e falhas parciais.
- CLI e TUI compartilham estados sem obrigar o mesmo layout.

## Resultado estruturado

Quando aplicável, um resultado pode conter:

| Campo | Uso |
| --- | --- |
| `status` | Estado final: `success`, `warning`, `failed`, `skipped` ou `canceled`. |
| `summary` | Mensagem curta para leitura humana. |
| `count` | Totais da operação, quando houver múltiplos itens. |
| `data` | Dados estruturados específicos do comando. |
| `outputs` | Arquivos e artefatos gerados. |
| `warnings` | Avisos não fatais. |
| `errors` | Falhas, incluindo falhas parciais. |

## Modos e canais

- Modo padrão: resumo humano em `stdout`; avisos e erros em `stderr`.
- `--verbose`/`-v`: mantém o resumo e acrescenta diagnósticos, progresso detalhado e informações por item em `stderr`.
- `--quiet`: suprime sucesso e progresso; erros continuam em `stderr`.
- `--json`: emite exatamente um resultado JSON em `stdout`; diagnósticos continuam em `stderr`.
- `--quiet --json` é inválido, pois combina dois contratos incompatíveis.
- Comandos interativos rejeitam `--quiet` e `--json` quando o modo solicitado impediria a interação.

Os códigos de saída continuam compatíveis com o contrato existente: `0` para sucesso, `1` para erro de uso ou execução e códigos específicos já definidos pelo comando.

## Estados

- `success`: operação concluída sem ressalvas.
- `warning`: concluída com avisos ou falhas parciais não fatais.
- `failed`: não produziu o resultado esperado.
- `skipped`: item ou etapa não executada por uma condição conhecida.
- `canceled`: operação interrompida pelo usuário ou pelo sistema.

## Referência dos comandos

Os caminhos canônicos são:

- `caramel docx images list` e `caramel docx images extract`
- `caramel image colorize` e `caramel image generate`
- `caramel print 2up` e `caramel print cards`
- `caramel routine consolidate`
- `caramel config show`, `caramel config models list` e `caramel config models select`
- `caramel install`, `caramel version`, `caramel guide` e `caramel workspace`

Aliases legados permanecem documentados em [COMMANDS.md](COMMANDS.md), mas novos exemplos devem usar os caminhos canônicos.
