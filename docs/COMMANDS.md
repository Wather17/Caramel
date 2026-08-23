# 📖 Referência de Comandos do Caramel CLI

> **A lista completa e atualizada de comandos é gerada ao vivo pela própria CLI:**
> ```bash
> caramel guide            # lista todos os comandos por categoria
> caramel guide <termo>    # busca por caso de uso (ex: 'caramel guide caça-palavras')
> caramel <comando> --help # detalhes, contexto pedagógico, exemplos e flags
> ```
>
> Este documento descreve apenas a **organização** e as **compatibilidades** — não precisa
> ser mantido sincronizado com a árvore de comandos.

---

## 🗂️ Organização por Fluxo Pedagógico

Os comandos são agrupados por **fluxo pedagógico**, não por tipo de arquivo:

### 📄 Documentos Word (.docx)
| Comando | Finalidade |
| :--- | :--- |
| `caramel docx extract` | Extrai, lista ou colore imagens contidas em um `.docx` |

### 🎨 Imagens
| Comando | Finalidade |
| :--- | :--- |
| `caramel image colorize` | Colora imagens soltas, pastas ou `.docx` inteiros via IA |
| `caramel image generate` | Gera ilustrações e coleções pedagógicas em lote |

### 🖨️ Impressão & Papel
| Comando | Finalidade |
| :--- | :--- |
| `caramel print 2up` | Monta PDF A4 Paisagem com 2 atividades por folha |
| `caramel print cards` | Gera fichas pedagógicas A4 (PDF ou HTML) |

### 📅 Rotinas de Aula
| Comando | Finalidade |
| :--- | :--- |
| `caramel routine process` | Consolida rotinas semanais e classifica Campos de Experiência da BNCC |

### ⚙️ Configurações & IA
| Comando | Finalidade |
| :--- | :--- |
| `caramel config setup` | Assistente interativo de configuração de chaves |
| `caramel config set` | Define o valor de uma chave diretamente |
| `caramel config show` | Exibe local do arquivo e status das chaves |

### ℹ️ Sistema & Utilidades
| Comando | Finalidade |
| :--- | :--- |
| `caramel guide` | Guia didático: lista comandos ou busca por termo |
| `caramel install` | Instala o Caramel CLI globalmente |
| `caramel version` | Exibe a versão atual do executável |

---

## 🔗 Compatibilidade (atalhos silenciosos)

Para não quebrar o hábito dos usuários, os comandos abaixo **também** funcionam na raiz,
embora o guia exiba apenas o caminho agrupado:

| Atalho legado | Equivale a |
| :--- | :--- |
| `caramel 2up` | `caramel print 2up` |
| `caramel cards` | `caramel print cards` |
| `caramel colorize` | `caramel image colorize` |
| `caramel generate` | `caramel image generate` |
| `caramel process` | `caramel image colorize` (aliases `process`, `pipeline`, `run`) |

> O comando `caramel process` foi **absorvido** pelo `colorize`: ambos executavam exatamente o
> mesmo pipeline (extrair → colorir → reconstruir o `.docx`). O `colorize` agora é o comando
> canônico e os nomes antigos seguem funcionando como aliases.

---

## ✨ Exemplo de Fluxo Completo

```bash
# 1. Gerar uma coleção de imagens de frutas
caramel image generate --items "maçã, banana, uva" -s clipart

# 2. Diagramar fichas A4 para impressão
caramel print cards ./imagens_frutas/

# 3. Montar uma atividade 2 por folha
caramel print 2up ./imagens_frutas/
```

Para criar um **novo comando**, siga as políticas de nomes, flags e documentação em
[`docs/CONTRIBUTING_COMMANDS.md`](CONTRIBUTING_COMMANDS.md).