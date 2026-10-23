# 📦 Política e Processo de Releases

Este documento define os critérios, regras de versionamento e o passo a passo para gerar novas versões (Releases) oficiais do **Caramel CLI**.

---

## 🚦 Critérios para Lançamento de Release

Antes de gerar uma nova tag e disparar o build automatizado, garanta que todos os critérios abaixo foram atendidos:

1. **Testes Verificados**: Todos os testes locais devem passar sem falhas:
   ```bash
   go test -race ./...
   go vet ./...
   CGO_ENABLED=0 go build ./...
   ```
2. **Ambiente Limpo**: O branch `main` deve estar atualizado e sem alterações pendentes (`git status` limpo).
3. **Documentação Atualizada**: Se a organização, os fluxos ou as políticas públicas mudaram, atualize o [`docs/COMMANDS.md`](./COMMANDS.md) e os guias afetados. A lista detalhada de comandos é derivada ao vivo pela própria CLI.
4. **Build Local de Validação**: O script de build local não deve retornar erros:
   ```bash
   ./scripts/build.sh
   ```
5. **Instalador Linux**: O instalador deve continuar detectando `amd64` e `arm64`, instalando sem
   privilégios administrativos, preservando aliases existentes e passando o smoke test isolado:
   ```bash
   bash scripts/test-install.sh
   ```
   O harness também cobre reinstalação, atualização, PATH idempotente, assets ausentes,
   checksum inválido e arquitetura incompatível usando fixtures locais. O workflow de tag executa
   `CARAMEL_ARTIFACT_DIR=dist bash scripts/test-install.sh` depois de gerar `SHA256SUMS` e antes de
   publicar os artefatos.

---

## 🏷️ Padrão de Versionamento (SemVer)

O Caramel CLI segue o padrão **Semantic Versioning 2.0.0** (`vMAJOR.MINOR.PATCH`):

- **MAJOR** (`v1.0.0`): Alterações incompatíveis com versões anteriores (breaking changes).
- **MINOR** (`v0.3.0`): Novas funcionalidades adicionadas de forma retrocompatível.
- **PATCH** (`v0.2.1`): Correções de bugs (bug fixes) e otimizações retrocompatíveis.

---

## 🚀 Passo a Passo para Lançar uma Release

Após validar todos os critérios, siga estes comandos no Linux Mint ou em um runner Ubuntu. O WSL pode
ser usado para repetir o fluxo Linux dentro da distribuição, mas não é o caminho principal de suporte.

### 1. Criar a Tag de Versão
Crie uma tag anotada descrevendo as principais mudanças:
```bash
git tag -a v0.2.0 -m "feat: adiciona coloração inteligente de imagens e rebuilder de docx"
```

### 2. Enviar a Tag para o GitHub
Empurre a tag para o repositório remoto para disparar a Action:
```bash
git push origin v0.2.0
```

### 3. Acompanhar e Baixar
1. Acesse a aba **Actions** do seu repositório no GitHub para ver o progresso do build.
2. Assim que concluir (aproximadamente 1-2 minutos), vá na aba **Releases**.
3. Baixe o executável correspondente ao seu sistema ou use o instalador Linux publicado no repositório.
   O arquivo `SHA256SUMS` é publicado junto dos binários para validação.

## ✅ Matriz de validação automática e manual

| Fluxo | Validado automaticamente | Ainda exige validação manual |
| --- | --- | --- |
| Pull request | Testes Go com race, vet, build Linux amd64/arm64 e Windows amd64, além do smoke test do instalador Linux em `HOME` temporário | Experiência visual em uma sessão Linux Mint e execução real em hardware ARM64 |
| Tag `v*` | Build dos três artefatos, smoke test de `dist`, geração de `SHA256SUMS`, publicação da release e atualização do manifesto Scoop | Conferir download, checksum e abertura de uma nova sessão em cada ambiente suportado |
| Linux ARM64 | Formato ELF, nome do artefato e arquitetura cruzada são verificados; o smoke test não executa o binário ARM64 no host amd64 | Execução real em ARM64 nativo ou emulação, quando o comportamento de runtime for relevante |
| Windows | Compilação amd64 e manifesto Scoop são produzidos | Instalação PowerShell, `caramel`/`mel`, PATH e integração do PowerShell em runner Windows |

O smoke test Linux usa diretórios temporários e não modifica a máquina nem a biblioteca do usuário.
Falhas de checksum, arquitetura ou artefato ausente devem ser reproduzidas antes da publicação e
registradas na issue/PR correspondente.
