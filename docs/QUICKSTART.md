# Início rápido (self-hosted)

Do zero ao primeiro atendimento em poucos minutos.

## Requisitos

- **Servidor VPS/dedicado com Ubuntu 24.04 ou 26.04** e **Docker** (o
  instalador instala o Docker automaticamente se faltar). Este é o ambiente
  **suportado** — o projeto foi projetado para rodar assim.
- Para usar um domínio com HTTPS: o DNS do domínio já apontando para o servidor
  (portas 80 e 443 liberadas).

> **Rodar em máquina local** (`localhost`, Windows/macOS/desktop) é possível,
> mas **por sua conta e risco**. Não é um ambiente suportado: o sistema resolve
> o tenant pelo domínio e depende de HTTPS/domínio/portas públicas para operar
> de ponta a ponta.

### Integrações necessárias

- **WhatsApp:** o canal WhatsApp usa a **Notreve WhatsApp API** (a infraestrutura
  é administrada pela Notreve). Para conectar um número, **contrate o serviço**
  em [whatsapp.notreve.com.br](https://whatsapp.notreve.com.br). Webchat e
  Telegram não dependem dele.
- **IA:** use qualquer provedor compatível com OpenAI; **recomendamos a
  [Notreve IA](https://ia.notreve.com.br)** (só o token).

## Instalar (um comando)

```bash
curl -sSL https://raw.githubusercontent.com/notrevetecnologia/notreve-chat-community/main/install.sh | sh
```

O instalador:

1. (se preciso) instala o Docker;
2. baixa a versão atual (branch `main`);
3. **gera os segredos** sozinho (chave mestra, HMAC, senhas);
4. pergunta o **domínio** do painel — **Enter** usa `localhost` (sem TLS);
5. sobe os containers e espera o painel ficar saudável;
6. mostra a **URL**, o usuário (`super_admin`) e a **senha gerada** (uma única vez).

> Guarde a senha exibida. Ela não é mostrada novamente; troque depois no painel.

Ao final, o atalho `ncc` fica disponível para gerenciar a instalação.

## Primeiro acesso

1. Abra a URL mostrada e entre com `super_admin` + a senha gerada.
2. Ative a **verificação em duas etapas (MFA)** quando solicitado.
3. Como **super_admin**, cadastre a **API WhatsApp** (base_url + token) e crie a
   **empresa + o primeiro admin**.
4. Entre como o **admin da empresa** e rode o assistente:
   **Provedor de IA → Agente → Canal**.

Sem um provedor de IA configurado a automação não responde; o assistente guia
esse passo.

## Gerenciar

```bash
ncc up            # sobe os containers
ncc down          # derruba os containers
ncc restart       # reinicia
ncc logs app      # logs ao vivo (troque app por outro servico)
ncc backup        # backup do banco em backups/
ncc restore backups/pg-AAAAMMDD-HHMMSS.sql
ncc doctor        # diagnostico (docker, compose, health, disco)
ncc version       # versao instalada
```

## Atualizar

```bash
sudo ncc update
```

O update faz **backup do banco**, puxa a versão atualizada, aplica as migrations e
sobe de novo. Se algo falhar, o backup fica em `backups/` para restaurar com
`ncc restore`.

## Instalação manual (a partir do código)

```bash
git clone https://github.com/notrevetecnologia/notreve-chat-community.git
cd notreve-chat-community
sudo ./install.sh          # ou: sudo ./install.sh --domain painel.exemplo.com
```

Para um ambiente de desenvolvimento sem Docker, veja `docs/SETUP.md`.

## Opções do instalador

| Opção | Descrição |
|---|---|
| `--dir <caminho>` | Diretório de instalação (padrão `/opt/notreve-chat-community`) |
| `--domain <host>` | Domínio do painel (padrão: localhost em HTTP) |
| `--update` | Atualiza uma instalação existente |
| `--reset` | Remove containers e volumes (**apaga os dados**) |
| `--yes` | Não-interativo (tudo automático, domínio = localhost) |
| `--ref <ref>` | Branch, tag ou commit (padrão `main`) |
| `-h`, `--help` | Ajuda |

## Problemas comuns

- **O painel não abre com domínio:** confira o DNS apontando para o servidor e
  as portas 80/443 liberadas. O Caddy emite o certificado automaticamente.
- **`docker compose` não encontrado:** instale o `docker-compose-plugin`.
- **Quero reinstalar do zero:** `sudo ./install.sh --reset` (apaga os dados) e
  depois `sudo ./install.sh`.
- **Ver o que está acontecendo:** `ncc doctor` e `ncc logs`.
