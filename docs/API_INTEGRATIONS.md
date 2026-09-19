# Contrato de integrações com APIs externas

Este documento é o contrato de avaliação para chamadas de serviços externos no Caramel. O provedor remoto atual é o OpenRouter. Qualquer nova integração deve seguir estas regras e atualizar a matriz antes de ser implementada.

## Objetivos

- manter chamadas canceláveis, limitadas e testáveis;
- diferenciar falhas de transporte, provedor, contrato e conteúdo;
- preservar resultados parciais e ordem determinística em lotes;
- tornar retry, fallback, custo e idempotência decisões explícitas;
- evitar que segredos, respostas brutas ou URLs inseguras cheguem ao armazenamento ou ao diagnóstico normal.

## Operações atuais

| Operação | Entrada e endpoint | Saída esperada | Custo potencial | Retry/concurrency atual | Fallback atual | Persistência atual |
| --- | --- | --- | --- | --- | --- | --- |
| Catálogo | `GET https://openrouter.ai/api/v1/models`; não exige chave | páginas de modelos, preço e modalidades | sem cobrança de geração | páginas sequenciais; até 100 páginas; retry transitório até 3 tentativas; timeout de 60 s | nenhum | apenas usado para seleção; não grava tentativas |
| Síntese de prompts | `POST /api/v1/chat/completions` com modelo de texto e texto de entrada | conteúdo textual contendo array de itens com `name` e `prompt` | depende do modelo de texto | uma chamada lógica com até 3 tentativas; limitador por cliente; `Retry-After` respeitado; workflows propagam o contexto até a chamada HTTP | fallback opcional configurado por `MODEL_TEXT_FALLBACKS`, uma troca máxima por item | execução guarda opções e erro agregado |
| Geração de imagem | `POST /api/v1/chat/completions` com modalidade `image` e `image_config` | bytes de imagem inline ou URL/data URL no envelope | normalmente cobrada por imagem/modelo | harness indexado; workers adaptativos de 1--5 conforme lote; teto compartilhado de 4 requests por cliente; retry seguro só antes do envio | resultado ambíguo após envio não repete nem usa fallback; fallback configurado só cobre falhas elegíveis antes do envio | salva imagem, artefato e eventos redigidos de tentativa |
| Triagem | `POST /api/v1/chat/completions` com imagem e prompt de decisão | objeto com `should_colorize` e `reason` | baixo ou gratuito, conforme modelo | até 2 tentativas dentro da colorização; limitador compartilhado; `Retry-After` respeitado | fallback opcional configurado por `MODEL_TRIAGE_FALLBACKS`; falha final continua fail-open | registra apenas resultado agregado da colorização |
| Colorização | `POST /api/v1/chat/completions` com imagem, prompt e modalidade `image` | bytes de imagem colorida | normalmente cobrada por imagem/modelo | lote indexado com workers adaptativos; cliente compartilhado por lote; teto de 4 requests; retry seguro só antes do envio | resultado ambíguo após envio não repete nem usa fallback; reutiliza `MODEL_IMAGE_FALLBACKS` apenas para falhas elegíveis | salva artefato/derivação e eventos redigidos de tentativa |

O comportamento listado como atual é o baseline. As lacunas restantes são implementadas nas issues #74--#78; esta documentação acompanha o runtime de #72 e #73.

## Regras normativas

### 1. Transporte

- Toda chamada deve usar `context.Context`, timeout explícito e `defer resp.Body.Close()`.
- Autenticação deve ser enviada somente no header `Authorization`; nunca em query string, logs ou artefatos.
- Corpos de resposta devem ser lidos com limite antes de decodificar JSON ou texto. O cliente deve validar `Content-Length` quando presente.
- O endpoint, método e modalidade devem ser definidos pelo cliente, nunca derivados de texto retornado pelo modelo.
- Downloads de imagens retornadas pela API devem aceitar somente URLs permitidas pela política de segurança, revalidando cada redirect.

### 2. Classes de falha

| Classe | Exemplos | Ação padrão |
| --- | --- | --- |
| Transporte transitório | timeout, conexão, DNS, 408, 429, 5xx | retry conforme a operação, backoff com jitter e cancelamento |
| Provedor permanente | 400, 401, 403, 404, 422, chave ausente ou modalidade incompatível | encerrar sem retry; mensagem acionável |
| Contrato de saída | JSON inválido, texto no lugar de array/objeto, campos ausentes, payload vazio | validar com parser/schema; fallback somente se elegível |
| Conteúdo/recurso | imagem inválida, corpo grande, URL bloqueada, disco cheio | encerrar sem retry/fallback automático |
| Cancelamento | `context.Canceled` ou deadline | interromper espera e chamadas pendentes; preservar sucessos anteriores |

O erro final deve manter a causa e informar operação, modelo efetivo, classe e item quando houver. O modo normal não deve exibir corpo bruto.

### 3. Retry

- Cada operação declara o máximo de tentativas e se uma chamada é segura para repetir.
- Retry usa backoff exponencial com jitter e teto de 30 s. Em 408/429/5xx, respeita `Retry-After` válido em segundos ou data HTTP quando estiver dentro do teto; espera deve ser interrompida por contexto.
- Falha permanente, erro de contrato não recuperável, erro de segurança/recurso e cancelamento não são repetidos.
- Uma falha de transporte depois de uma operação cobrada ter sido aceita é resultado ambíguo. Sem idempotência confirmada, não repetir cegamente; retornar `unknown_outcome` (#78).
- Respostas e erros devem preservar status HTTP, request ID e metadados de rate limit para diagnóstico redigido (#74, #76).

### 4. Workers e rate limit

- A unidade de concorrência é um item independente do lote. Resultados e erros devem retornar na ordem de entrada.
- `--workers` é teto de trabalho, nunca bypass de um limitador compartilhado por cliente/provedor/operação.
- O catálogo mantém paginação sequencial porque a próxima URL depende da página anterior.
- O executor deve continuar itens independentes após falha, retornar sucesso parcial e cancelar workers pendentes quando o contexto terminar.
- O limitador compartilhado por cliente admite 4 requests simultâneos por padrão e é adquirido antes de cada request HTTP real, inclusive retries; não criar um pool independente por etapa.

### 5. Custo e idempotência

- A matriz de cada operação deve declarar `safe_retry`, cobrança potencial e comportamento para resultado ambíguo.
- Geração e colorização podem gerar cobrança por tentativa. Use correlação determinística por item e configuração, sem incluir API key ou prompt completo.
- Geração e colorização enviam `Idempotency-Key` e `X-Caramel-Correlation-ID` derivados por hash de item/configuração, sem API key ou prompt completo, e capturam request ID quando retornado.
- Falha de transporte depois do envio, 408 ou 5xx em operação cobrada vira `unknown_outcome`; não repetir nem acionar fallback sem confirmação externa (#78).
- Só grave artefatos no cache depois de validar bytes, formato e metadados. Reutilização local deve ser marcada como tal e não contar como novo consumo.

### 6. Saída e análise

- Decodifique primeiro o envelope do provedor e depois valide o contrato específico da operação.
- O parser deve diferenciar JSON puro, bloco Markdown válido, texto sem JSON, JSON truncado, tipo errado e schema incompleto.
- Síntese exige lista não vazia de itens com `name` e `prompt`; rotina exige os campos definidos pelo relatório; triagem exige `should_colorize` explícito e `reason` quando necessário.
- Um erro de contrato deve identificar operação, modelo e formato esperado. `--verbose` pode mostrar somente trecho truncado e redigido.
- O modo padrão mantém resumo em `stdout`, diagnósticos em `stderr`; `--json` emite um único resultado parseável sem resposta bruta.

### 7. Fallback

- Fallback é uma cadeia explícita por papel: texto, imagem e triagem. A lista é opt-in, ordenada e limitada a uma troca por item (#73).
- Só são elegíveis erro transitório esgotado ou erro de contrato classificado como recuperável.
- Não fazer fallback para autenticação, entrada inválida, modalidade incompatível, erro permanente, segurança, recurso ou cancelamento.
- Toda transição informa modelo primário, modelo efetivo e ordinal da troca em diagnóstico e resultado estruturado.

### 8. Persistência e diagnóstico

- Persistir por item e tentativa: operação, papel/modelo solicitado e efetivo, ordinal de fallback, tempos, status, classe de erro, status HTTP, `Retry-After`, request ID, usage/custo quando fornecido e indicação de reutilização (#76). O registro usa `schema_version` e permanece legível quando os campos não existem em manifestos antigos.
- O cliente recebe um coletor opcional; workspace grava os eventos redigidos no manifesto e vault usa a tabela `run_attempts`. `--json` expõe a lista `attempts` sem prompt, corpo HTTP ou credencial.
- Não persistir API key, prompt completo, corpo HTTP, resposta bruta ou tokens de autenticação.
- Execuções parciais e canceladas conservam itens concluídos e estado terminal.
- O schema local deve ser versionado e ler registros antigos sem apagar informação silenciosamente.

### 9. Segurança e limites

- Envelopes JSON/texto de respostas têm limite de 4 MiB; downloads de imagens têm limite de 25 MiB e leitura limitada interrompe o corpo antes do decode/persistência.
- URLs remotas de imagem exigem HTTPS, permitem no máximo três redirects e validam novamente cada destino; loopback, RFC1918, link-local, multicast e endpoints de metadados são bloqueados.
- URLs remotas devem usar HTTPS por padrão, limitar redirects e bloquear loopback, RFC1918, link-local, multicast e endpoints de metadados; cada destino é revalidado.
- Data URLs permitidas continuam sujeitas a MIME, tamanho e bytes mágicos.
- Erros de limite, SSRF ou formato inválido não entram em retry/fallback.
- Logs e `--verbose` devem truncar e redigir conteúdo potencialmente sensível (#75).

### 10. Testes

Cada cliente deve ser testado com servidor fake determinístico e relógio/espera injetáveis quando necessário. A matriz mínima cobre:

- sucesso, JSON válido, cercas Markdown e payload de imagem inline;
- JSON inválido, truncado, vazio, schema incompleto e texto inesperado;
- rede, timeout, 408, 429 com `Retry-After`, 5xx e erro permanente;
- cancelamento durante request, backoff, paginação e workers;
- concorrência máxima, ordem, sucesso parcial e falha de um item;
- fallback elegível e não elegível, modelo efetivo e limite de uma troca;
- corpo grande, download grande, redirect, endereço privado, Data URL e bytes mágicos inválidos;
- idempotência, resultado ambíguo, cache válido/corrompido, redaction e migração de persistência.

## Estado de implementação e rastreabilidade

| Regra | Estado | Issue |
| --- | --- | --- |
| Executor indexado, cancelável e sucesso parcial | implementado | #45, #46, #47 |
| Catálogo sequencial, retry e limite de páginas | implementado | #48 |
| Parser e validação de saída estruturada | implementado | #72 |
| Fallback por papel | implementado | #73 |
| `Retry-After` e limitador compartilhado | implementado | #74 |
| Limites de corpo e URLs seguras | implementado | #75 |
| Metadados de tentativa/uso persistidos | implementado | #76 |
| Contexto nos workflows | implementado | #77 |
| Idempotência e resultado ambíguo | implementado | #78 |

Novas integrações devem adicionar uma linha à matriz, declarar todas as dez dimensões e apontar para testes que demonstrem cada limite.
