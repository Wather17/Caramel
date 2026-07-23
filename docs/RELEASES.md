# 📦 Política e Processo de Releases

Este documento define os critérios, regras de versionamento e o passo a passo para gerar novas versões (Releases) oficiais do **Caramel CLI**.

---

## 🚦 Critérios para Lançamento de Release

Antes de gerar uma nova tag e disparar o build automatizado, garanta que todos os critérios abaixo foram atendidos:

1. **Testes Verificados**: Todos os testes locais devem passar sem falhas:
   ```bash
   go test -v ./...
   ```
2. **Ambiente Limpo**: O branch `main` deve estar atualizado e sem alterações pendentes (`git status` limpo).
3. **Documentação Atualizada**: Se novos comandos ou flags foram adicionados, eles devem constar no [`docs/COMMANDS.md`](./COMMANDS.md).
4. **Build Local de Validação**: O script de build local não deve retornar erros:
   ```bash
   ./scripts/build.sh
   ```

---

## 🏷️ Padrão de Versionamento (SemVer)

O Caramel CLI segue o padrão **Semantic Versioning 2.0.0** (`vMAJOR.MINOR.PATCH`):

- **MAJOR** (`v1.0.0`): Alterações incompatíveis com versões anteriores (breaking changes).
- **MINOR** (`v0.3.0`): Novas funcionalidades adicionadas de forma retrocompatível.
- **PATCH** (`v0.2.1`): Correções de bugs (bug fixes) e otimizações retrocompatíveis.

---

## 🚀 Passo a Passo para Lançar uma Release

Após validar todos os critérios, siga estes comandos no seu terminal do WSL:

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
3. Baixe o executável correspondente ao seu sistema (ex: `caramel-windows-amd64.exe`).
