#!/usr/bin/env bash

# Garante que o script pare em caso de erros simples
set -e

# Define o diretório de destino
ISSUES_DIR="issues"

# Cria a pasta de issues se não existir
mkdir -p "$ISSUES_DIR"

# Limpa a pasta local para refletir apenas as issues ativas do repositório remoto
rm -f "$ISSUES_DIR"/*.md

echo "Sincronizando issues abertas do GitHub..."

# Verifica se o gh CLI está instalado
if ! command -v gh &> /dev/null; then
  echo "Erro: GitHub CLI (gh) não está instalado ou não está no PATH."
  exit 1
fi

# Obtém a lista de números das issues abertas
issues=$(gh issue list --state open --json number --jq '.[].number' 2>/dev/null || true)

if [ -z "$issues" ]; then
  echo "Nenhuma issue aberta encontrada ou erro de autenticação/conexão com o GitHub."
  exit 0
fi

for num in $issues; do
  # Obtém o título da issue
  title=$(gh issue view "$num" --json title --jq '.title' 2>/dev/null || echo "issue-$num")

  # Cria um slug simples e seguro para o nome do arquivo
  slug=$(echo "$title" | tr '[:upper:]' '[:lower:]' \
                    | sed 's/ /-/g' \
                    | sed 's/[^a-z0-9-]//g' \
                    | sed 's/-\+/-/g' \
                    | cut -c1-40) # limita tamanho do slug

  filename="${ISSUES_DIR}/${num}-${slug}.md"

  echo " -> Sincronizando: #${num} - $title"

  # Obtém as labels associadas
  labels=$(gh issue view "$num" --json labels --jq '[.labels[].name] | join(", ")' 2>/dev/null || echo "")

  # Gera o arquivo Markdown completo
  {
    echo "# Issue #${num}: ${title}"
    if [ -n "$labels" ]; then
      echo "**Labels**: $labels"
    fi
    echo ""
    echo "## Descrição"
    gh issue view "$num" --json body --jq '.body' 2>/dev/null
    echo ""

    # Obtém e formata comentários se existirem
    comments=$(gh issue view "$num" --json comments --jq '.comments[] | "### Comentário por @\(.author.login):\n\(.body)\n"' 2>/dev/null || echo "")
    if [ -n "$comments" ]; then
      echo "## Discussão"
      echo "$comments"
    fi
  } > "$filename"
done

echo "Sincronização concluída com sucesso! $(find "$ISSUES_DIR" -name "*.md" | wc -l) issues ativas salvas em ./${ISSUES_DIR}/"
