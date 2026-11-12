# Evidência de uso para decisões do Caramel

O `caramel usage` é uma fonte local de evidência para revisar o produto. Ele ajuda a
perceber quais fluxos fazem parte do uso diário, quais flags quase nunca aparecem e
onde há falhas recorrentes, mas não decide sozinho o que deve ser removido.

## Coletar um relatório

Use uma janela explícita e prefira JSON quando o resultado for entrar em uma issue:

```bash
mel usage --days 90 --json
```

Para leitura rápida no terminal:

```bash
mel usage --days 30
mel usage --all-time
```

O histórico é local ao vault do Caramel. A coleta registra somente o caminho canônico
do comando, nomes de flags explicitamente usados, data, duração e resultado. Valores de
flags, argumentos, caminhos de arquivos, conteúdo de materiais e telemetria externa não
fazem parte do relatório.

## Como interpretar

Antes de propor uma mudança, leia primeiro a seção `coverage` do JSON ou a seção
`Cobertura da evidência` do relatório humano:

- `events` é o número de tentativas observadas no período.
- `active_days` mostra em quantos dias houve execução.
- `observed_days` mostra o intervalo coberto pelos eventos encontrados.
- `unknown` representa eventos antigos ou execuções que não chegaram a ser finalizadas.
- comandos e flags com contagem zero são itens do inventário que não apareceram na janela.

Uma baixa contagem pode significar pouca demanda, sazonalidade, descoberta ruim,
complexidade ou falha de execução. Use `last_used`, status, duração, tendência e o
contexto real do fluxo antes de chamar algo de obsoleto.

Os sinais de revisão são fatos para investigação humana:

- `never_used_command` e `never_used_flag`: não houve ocorrência na janela consultada;
- `failed_command`: houve pelo menos uma execução com falha;
- `older_than_half_window`: o último uso ficou na primeira metade da janela;
- `insufficient_data`: não há evento suficiente para observar o fluxo no período.

Nenhum sinal significa automaticamente manter, simplificar ou remover um comando.
Uma feature nova também não deve ser bloqueada apenas porque ainda não existe uso
histórico dela.

## Como anexar a uma issue

Inclua um bloco curto na descrição ou comentário da issue, separando observação de
interpretação:

```markdown
## Evidência de uso local

Comando consultado: `mel usage --days 90 --json`
Data da coleta: AAAA-MM-DD

### Observação
- [comandos, flags, status, tendência e cobertura observados]

### Hipótese
- [o que esses dados podem indicar]

### Decisão proposta
- [melhorar, simplificar, manter, investigar ou criar a feature]

### Limitações
- [janela curta, poucos dias ativos, sazonalidade, dados `unknown` ou ausência de uso]
```

O bloco é evidência complementar. A issue continua precisando de problema, comportamento
esperado, escopo, critérios de aceite e validação independentes do relatório de uso.
