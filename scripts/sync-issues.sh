#!/usr/bin/env bash

set -Eeuo pipefail

ISSUES_DIR="${ISSUES_DIR:-issues}"
tmp_dir=""

cleanup() {
  if [[ -n "$tmp_dir" && -d "$tmp_dir" ]]; then
    rm -rf -- "$tmp_dir"
  fi
}
trap cleanup EXIT

if ! command -v gh >/dev/null 2>&1; then
  printf 'Erro: GitHub CLI (gh) não está instalado ou não está no PATH.\n' >&2
  exit 1
fi

mkdir -p -- "$ISSUES_DIR"

printf 'Sincronizando issues abertas do GitHub...\n'

# O cache atual só é substituído depois que a consulta e o processamento de
# todas as issues terminarem com sucesso.
tmp_dir=$(mktemp -d "${ISSUES_DIR}.tmp.XXXXXX")
issue_data="$tmp_dir/issues.tsv"

# Uma única consulta traz todos os dados necessários. Cada campo é codificado
# individualmente para que tabs e quebras de linha do título, corpo ou
# comentários não quebrem o protocolo TSV consumido pelo Bash abaixo.
if ! gh issue list \
  --state open \
  --limit 1000 \
  --json number,title,body,labels,comments \
  --jq '.[] | [(.number | tostring), (.title // ""), (.labels | map(.name) | join(", ")), (.body // ""), (.comments | map("### Comentário por @\(.author.login):\n\(.body)\n") | join("\n"))] | map(@base64) | @tsv' \
  > "$issue_data"; then
  printf 'Erro: não foi possível consultar as issues abertas. Verifique a autenticação e a conexão.\n' >&2
  exit 1
fi

if printf '' | base64 --decode >/dev/null 2>&1; then
  base64_decode=(base64 --decode)
else
  base64_decode=(base64 -D)
fi

decode_base64() {
  printf '%s' "$1" | "${base64_decode[@]}"
}

count=0

while IFS= read -r record || [[ -n "$record" ]]; do
  # O registro é produzido pelo --jq acima, portanto deve sempre conter
  # exatamente quatro tabs separando os cinco campos codificados.
  record_without_tabs="${record//$'\t'/}"
  tab_count=$((${#record} - ${#record_without_tabs}))
  if ((tab_count != 4)); then
    printf 'Erro: resposta inválida ao consultar as issues abertas.\n' >&2
    exit 1
  fi

  IFS=$'\t' read -r encoded_num encoded_title encoded_labels encoded_body encoded_comments <<< "$record"
  if ! num=$(decode_base64 "$encoded_num") || ! title=$(decode_base64 "$encoded_title") \
    || ! labels=$(decode_base64 "$encoded_labels") || ! body=$(decode_base64 "$encoded_body") \
    || ! comments=$(decode_base64 "$encoded_comments"); then
    printf 'Erro: resposta inválida ao consultar os dados das issues abertas.\n' >&2
    exit 1
  fi

  slug=$(printf '%s' "$title" \
    | LC_ALL=C tr '[:upper:]' '[:lower:]' \
    | tr ' ' '-' \
    | LC_ALL=C sed -e 's/[^a-z0-9-]//g' -e 's/-\+/-/g' -e 's/^-*//' -e 's/-*$//' \
    | cut -c1-40)
  slug="${slug:-issue}"
  filename="${num}-${slug}.md"

  printf ' -> Sincronizando: #%s - %s\n' "$num" "$title"

  {
    printf '# Issue #%s: %s\n' "$num" "$title"
    if [[ -n "$labels" ]]; then
      printf '**Labels**: %s\n' "$labels"
    fi
    printf '\n## Descrição\n'
    printf '%s\n' "$body"
    printf '\n'

    if [[ -n "$comments" ]]; then
      printf '## Discussão\n'
      printf '%s\n' "$comments"
    fi
  } > "$tmp_dir/$filename"

  count=$((count + 1))
done < "$issue_data"

shopt -s nullglob
old_files=("$ISSUES_DIR"/*.md)
if ((${#old_files[@]} > 0)); then
  rm -f -- "${old_files[@]}"
fi

new_files=("$tmp_dir"/*.md)
if ((${#new_files[@]} > 0)); then
  mv -- "${new_files[@]}" "$ISSUES_DIR/"
fi

printf 'Sincronização concluída com sucesso! %d issues ativas salvas em ./%s/\n' "$count" "$ISSUES_DIR"
