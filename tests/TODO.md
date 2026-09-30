# TODO — validação prática das features

Este arquivo acompanha a revisão manual das features entregues nesta semana.
O baseline atual é a tag `v1.9.0`, no commit `e5b419d`.

## Como vamos usar

- `[ ]` pendente
- `[>]` em teste
- `[x]` aprovado na prática
- `[!]` comportamento incorreto ou dúvida que precisa virar issue

Para cada item, registrar nesta seção o comando executado, os arquivos usados, o
resultado observado e, quando necessário, a referência da issue criada com
`refine-issues`. Não considerar os testes automatizados como substitutos do teste
manual: eles apenas dão o ponto de partida.

## Preparação

- [x] Confirmar baseline instalado: `caramel version --json` e `mel version --json` retornam `1.9.0` / `e5b419d`.
- [ ] Separar arquivos reais ou cópias de teste para PDF, DOCX e imagens.
- [ ] Registrar sistema operacional, versão do Go/CLI e provedor/modelos usados nos fluxos que consomem API.
- [ ] Confirmar que cada teste que usa IA pode gerar custo antes de executá-lo.

## 1. PDF e impressão local — sem API

### 1.1 Criar PDF a partir de imagens — issue #54

- [ ] Executar `caramel pdf create ./imagens --output criado.pdf`.
- [ ] Conferir uma imagem por página, ordem natural da pasta, orientação retrato/paisagem, proporção sem corte e abertura no leitor de PDF.
- [ ] Repetir com imagens informadas explicitamente em ordem diferente.
- [ ] Confirmar que entrada inválida não deixa PDF parcial.

### 1.2 Juntar PDFs — issue #53

- [ ] Executar `caramel pdf merge capa.pdf atividades.pdf respostas.pdf --output apostila.pdf`.
- [ ] Conferir ordem das páginas, tamanho/orientação originais e conteúdo de cada arquivo.
- [ ] Tentar saída igual a uma entrada e confirmar rejeição sem sobrescrever o original.

### 1.3 Dividir PDFs por página e intervalo — issue #55

- [ ] Executar `caramel pdf split apostila.pdf` e conferir um PDF por página.
- [ ] Executar `caramel pdf split apostila.pdf --ranges 1-3,5,7-9 --output-dir ./capitulos`.
- [ ] Conferir que intervalos são inclusivos e preservam a ordem informada.
- [ ] Testar um intervalo inválido e confirmar que nenhum arquivo parcial é publicado.

### 1.4 Renderizar páginas completas — issue #56

- [ ] Executar `caramel pdf pages render apostila.pdf` e abrir as imagens resultantes.
- [ ] Repetir com `--format jpg --dpi 300 --output-dir ./paginas`.
- [ ] Conferir que texto, vetores e imagens aparecem na composição final e que a numeração segue a ordem original.

### 1.5 Extrair imagens embutidas — issue #57

- [ ] Executar `caramel pdf images extract apostila.pdf --output-dir ./imagens-extraidas`.
- [ ] Comparar as imagens extraídas com os recursos originalmente embutidos.
- [ ] Confirmar que texto e vetores não viram imagens e que arquivos existentes não são sobrescritos.
- [ ] Registrar os avisos produzidos por recursos que não puderem ser decodificados.

### 1.6 Ampliar formatos do `print 2up` — issue #14

- [ ] Testar PNG, JPG/JPEG, JPE, JFIF/JIF, WEBP, GIF, BMP e TIF/TIFF, incluindo extensões em maiúsculas.
- [ ] Conferir GIF animado usando somente o primeiro frame e conversão dos formatos não nativos em PDF.
- [ ] Confirmar rejeição de extensão não suportada e de conteúdo que não é imagem válida.

## 2. DOCX

### 2.1 Dividir por quebras explícitas — issue #64

- [ ] Criar ou usar um DOCX com quebras manuais de página e de seção e executar `caramel docx split apostila.docx`.
- [ ] Conferir partes na ordem, preservação do conteúdo e ausência de sobrescrita de saídas existentes.
- [ ] Confirmar que o comando não tenta inferir páginas visuais renderizadas.
- [ ] Testar quebra dentro de tabela/bloco aninhado e registrar se o erro é claro e não deixa saída parcial.

### 2.2 Juntar DOCX preservando referências — issue #63

- [ ] Executar `caramel docx merge capa.docx atividades.docx respostas.docx --output apostila.docx`.
- [ ] Conferir texto, tabelas, imagens, estilos, listas, relações e início de cada origem em nova seção/página.
- [ ] Testar saída já existente e confirmar que ela nunca é sobrescrita.
- [ ] Exercitar um documento com recurso fora do escopo (comentários, notas, revisão ou objeto incorporado) e registrar a mensagem de rejeição.

## 3. Geração, cache e processamento de imagens via IA

### 3.1 Reutilizar a biblioteca local — issue #69

- [ ] Gerar itens diretos com `caramel image generate --items "bolo, pão" --reuse-cache --no-preview`.
- [ ] Repetir a mesma solicitação e conferir reutilização local, sem nova geração, arquivos legíveis e saída determinística.
- [ ] Repetir com `--refresh-cache` e conferir nova versão ativa sem apagar a anterior.
- [ ] Variar conceito, tema, estilo, proporção, estilo customizado e modelo para confirmar que imagens visualmente diferentes não colidem no cache.
- [ ] Testar um acerto de cache sem chave de API e registrar se o fluxo realmente funciona offline.

### 3.2 Workers, ordem e sucesso parcial — issues #45–#47

- [ ] Executar `caramel image generate` com vários itens e `--workers 1`, depois com `--workers 2` ou mais; comparar ordem e artefatos.
- [ ] Executar `caramel image colorize pasta/ --workers 2` com itens válidos, já coloridos e inválidos; conferir continuação após falha e resumo parcial.
- [ ] Executar `caramel routine consolidate ./rotinas --workers 2`; conferir que cada rotina é processada uma vez e que o relatório mantém a ordem de entrada.
- [ ] Interromper um lote durante espera/requisição e conferir cancelamento sem corromper sucessos já concluídos.

### 3.3 Contexto da síntese e triagem

- [ ] Gerar itens por `--theme`, usando estilo e proporção explícitos, e conferir que o contexto chega à imagem final.
- [ ] Comparar `--reuse-cache` com mudanças de tema, estilo, proporção e estilo customizado; nenhum contexto diferente deve reutilizar imagem indevida.
- [ ] Em `image colorize`, testar imagem em preto e branco, imagem já colorida e atividade de colorir; conferir triagem, pulos e uso opcional de `--no-triage`.

## 4. Integrações de IA e confiabilidade — v1.9.0

Estes itens devem ser executados com um modelo/provedor controlado. Falhas simuladas,
respostas malformadas e URLs artificiais devem preferencialmente usar um servidor fake
local, para não gerar custo nem depender de comportamento instável do provedor.

### 4.1 Fallback configurável por papel — issue #81

- [ ] Configurar `MODEL_TEXT_FALLBACKS`, `MODEL_IMAGE_FALLBACKS` e `MODEL_TRIAGE_FALLBACKS` em ambiente de teste.
- [ ] Forçar falha recuperável do modelo primário e conferir uma única troca para o fallback, com modelo efetivo visível no diagnóstico.
- [ ] Confirmar que autenticação, entrada inválida, cancelamento, erro de segurança e resultado ambíguo não acionam fallback.
- [ ] Confirmar a precedência flag > `.env` > padrão e que modelos duplicados não geram tentativas extras.

### 4.2 Retry, `Retry-After` e limite compartilhado — issue #82

- [ ] Simular 408, 429 com `Retry-After`, timeout e 5xx e conferir backoff interrompível e número máximo de tentativas.
- [ ] Rodar um lote com `--workers` acima do limite e conferir que o limite de requests continua compartilhado pelo cliente.
- [ ] Conferir que a paginação de `config models list` permanece sequencial e pode ser cancelada com Ctrl-C.
- [ ] Confirmar sucesso parcial e ordem determinística quando apenas alguns itens falham.

### 4.3 Contrato estruturado e limites de resposta — issues #80 e #83

- [ ] Executar geração, rotina e triagem com `--json`; conferir resultado parseável e ausência de resposta bruta no modo normal.
- [ ] Simular JSON vazio, truncado, tipo errado, campos ausentes e texto sem JSON; conferir erro acionável e classificação correta.
- [ ] Simular corpo de resposta acima do limite e imagem acima do limite; confirmar rejeição sem retry/fallback automático.
- [ ] Confirmar que `--verbose` mostra apenas diagnóstico truncado e redigido.

### 4.4 Segurança de URLs e downloads — issue #83

- [ ] Com servidor fake, testar URL HTTP, loopback, RFC1918, link-local, multicast, endpoint de metadados e redirects excessivos.
- [ ] Confirmar bloqueio antes de persistir bytes e ausência de segredo, corpo bruto ou URL insegura nos logs/artefatos.
- [ ] Testar data URL permitida com MIME, tamanho e bytes mágicos válidos; rejeitar os casos inválidos.

### 4.5 Retry seguro e resultado ambíguo em imagens — issue #84

- [ ] Em servidor fake, conferir `Idempotency-Key` e `X-Caramel-Correlation-ID` estáveis por item/configuração e diferentes entre itens/configurações.
- [ ] Simular perda da resposta depois do envio e confirmar `unknown_outcome`, sem nova tentativa nem fallback automático.
- [ ] Conferir que retry ocorre somente antes do envio efetivo quando a operação é cobrada.
- [ ] Não executar este cenário contra uma conta paga sem servidor/request ID controlável.

### 4.6 Metadados de tentativas — issue #86

- [ ] Executar um fluxo com sucesso, retry, fallback e falha parcial usando `--json`.
- [ ] Conferir a lista `attempts`: operação, papel/modelo efetivo, ordinal, duração, status/classe, request ID, rate limit e indicação de reutilização quando disponíveis.
- [ ] Conferir persistência no manifesto/workspace e no vault, inclusive leitura de registro antigo.
- [ ] Confirmar que API key, prompt completo, corpo HTTP, resposta bruta e tokens não aparecem nos registros.

## Registro de resultados

| Data | Item | Resultado | Evidência | Issue/refino |
| --- | --- | --- | --- | --- |
|  |  |  |  |  |

## Próximo passo

Começar pela seção **1**, que não consome API, depois seguir para DOCX e cache.
Ao encontrar qualquer comportamento inesperado, parar o item correspondente,
registrar a reprodução nesta tabela e usar `refine-issues` antes de alterar o código.
