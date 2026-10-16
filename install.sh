#!/bin/sh
#
# Notreve Chat Community - instalador/atualizador self-hosted.
#
# Instalacao (self-hosted, um comando):
#   curl -sSL https://raw.githubusercontent.com/notrevetecnologia/notreve-chat-community/main/install.sh | sh
#
# Uso local (checkout do repositorio):
#   ./install.sh
#
# Opcoes:
#   --dir <caminho>   Diretorio de instalacao (padrao: /opt/notreve-chat-community)
#   --domain <host>   Dominio do painel (padrao: localhost em HTTP na porta 80)
#   --update          Atualiza uma instalacao existente (faz backup do banco antes)
#   --reset           Remove containers e volumes (APAGA OS DADOS)
#   --yes             Nao-interativo (gera tudo; dominio = localhost)
#   --ref <ref>       Branch, tag ou commit para instalar (padrao: main)
#   -h, --help        Mostra esta ajuda
#
set -eu

APP_NAME="Notreve Chat Community"
REPO="notrevetecnologia/notreve-chat-community"
GITHUB="https://github.com/${REPO}"
DEFAULT_DIR="/opt/notreve-chat-community"
HEALTH_RETRIES=300
HEALTH_SLEEP=2

MODE="auto"
DIR_ARG=""
DOMAIN=""
ASSUME_YES=0
VERSION="main"

log() { printf '==> %s\n' "$*"; }
warn() { printf 'aviso: %s\n' "$*" >&2; }
die() { printf 'erro: %s\n' "$*" >&2; exit 1; }

usage() {
	cat <<EOF
$APP_NAME - instalador self-hosted

Uso:
  curl -sSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh
  ./install.sh [opcoes]

Opcoes:
  --dir <caminho>   Diretorio de instalacao (padrao: $DEFAULT_DIR)
  --domain <host>   Dominio do painel (padrao: localhost em HTTP)
  --update          Atualiza uma instalacao existente
  --reset           Remove containers e volumes (APAGA OS DADOS)
  --yes             Nao-interativo
  --ref <ref>       Branch, tag ou commit (padrao: main)
  -h, --help        Esta ajuda
EOF
}

prompt() {
	if [ "$ASSUME_YES" -eq 1 ] || [ ! -r /dev/tty ]; then
		return 0
	fi
	printf '%s' "$1" > /dev/tty
	IFS= read -r _answer < /dev/tty || _answer=""
	printf '%s' "$_answer"
}

while [ $# -gt 0 ]; do
	case "$1" in
		--dir) DIR_ARG="${2:-}"; shift 2 ;;
		--dir=*) DIR_ARG="${1#*=}"; shift ;;
		--domain) DOMAIN="${2:-}"; shift 2 ;;
		--domain=*) DOMAIN="${1#*=}"; shift ;;
		--update) MODE="update"; shift ;;
		--reset) MODE="reset"; shift ;;
		--yes|-y) ASSUME_YES=1; shift ;;
		--ref) VERSION="${2:-}"; shift 2 ;;
		--ref=*) VERSION="${1#*=}"; shift ;;
		--version) VERSION="${2:-}"; shift 2 ;;
		--version=*) VERSION="${1#*=}"; shift ;;
		-h|--help) usage; exit 0 ;;
		*) die "opcao desconhecida: $1 (use --help)" ;;
	esac
done

INSTALL_SH_URL="${GITHUB}/raw/${VERSION}/install.sh"
TARBALL_URL="${GITHUB}/archive/${VERSION}.tar.gz"

fetch_text() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "$1"
	else
		die "curl ou wget e necessario para baixar arquivos"
	fi
}

fetch_file() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
	else
		die "curl ou wget e necessario para baixar arquivos"
	fi
}

if [ "$(id -u)" -ne 0 ]; then
	log "Elevando privilegios com sudo..."
	if [ -f "$0" ]; then
		exec sudo -E sh "$0" "$@"
	fi
	fetch_text "$INSTALL_SH_URL" | sudo -E sh -s -- "$@"
	exit $?
fi

SELF_IS_FILE=0
SCRIPT_DIR=""
if [ -f "$0" ]; then
	SELF_IS_FILE=1
	SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
fi

if [ -n "$DIR_ARG" ]; then
	DIR="$DIR_ARG"
elif [ "$SELF_IS_FILE" -eq 1 ] && [ -f "$SCRIPT_DIR/docker-compose.yml" ]; then
	DIR="$SCRIPT_DIR"
else
	DIR="$DEFAULT_DIR"
fi

EXISTING=0
if [ -f "$DIR/.env" ] && [ -f "$DIR/docker-compose.yml" ]; then
	EXISTING=1
fi

if [ "$MODE" = "update" ] && [ "$EXISTING" -eq 0 ]; then
	die "nenhuma instalacao encontrada em $DIR para atualizar"
fi

if [ "$MODE" = "reset" ]; then
	[ "$EXISTING" -eq 1 ] || die "nenhuma instalacao encontrada em $DIR"
	if [ "$ASSUME_YES" -eq 0 ]; then
		answer=$(prompt "Isto vai APAGAR todos os dados em $DIR. Digite \"reset\" para confirmar: ")
		[ "$answer" = "reset" ] || die "cancelado"
	fi
	cd "$DIR"
	log "Removendo containers e volumes..."
	docker compose down -v --remove-orphans
	log "Pronto. Os dados foram removidos."
	exit 0
fi

if [ -r /etc/os-release ]; then
	. /etc/os-release
	if [ "${ID:-}" != "ubuntu" ]; then
		warn "ambiente nao suportado: o projeto foi projetado para Ubuntu 24.04/26.04. Prossiga por sua conta e risco."
	fi
else
	warn "nao foi possivel detectar o sistema; o ambiente suportado e Ubuntu 24.04/26.04. Prossiga por sua conta e risco."
fi

if ! command -v docker >/dev/null 2>&1; then
	log "Docker nao encontrado."
	if [ "$ASSUME_YES" -eq 0 ]; then
		answer=$(prompt 'Instalar o Docker agora? [S/n] ')
		case "$answer" in
			n|N|nao|não) die "Docker e obrigatorio para continuar" ;;
		esac
	fi
	log "Instalando Docker (get.docker.com)..."
	fetch_text "https://get.docker.com" | sh
	systemctl enable --now docker >/dev/null 2>&1 || true
fi

if ! docker compose version >/dev/null 2>&1; then
	die "o plugin Docker Compose (v2) e obrigatorio (instale docker-compose-plugin)"
fi

download_tarball() {
	dest="$1"
	tmp=$(mktemp -d)
	log "Baixando ${APP_NAME} (${VERSION})..."
	fetch_file "$TARBALL_URL" "$tmp/app.tar.gz" || die "falha ao baixar $TARBALL_URL"
	mkdir -p "$dest"
	tar -xzf "$tmp/app.tar.gz" -C "$dest" --strip-components=1 || die "falha ao extrair o pacote"
	rm -rf "$tmp"
}

env_get() {
	grep -E "^$1=" "$DIR/.env" 2>/dev/null | head -n1 | cut -d= -f2-
}

rand_b64() {
	head -c "$1" /dev/urandom | base64 | tr -d '\n'
}

rand_pass() {
	head -c 32 /dev/urandom | base64 | tr -d '\n=+/' | cut -c1-24
}

mkdir -p "$DIR"

LOCAL_CHECKOUT=0
if [ "$SELF_IS_FILE" -eq 1 ] && [ "$SCRIPT_DIR" = "$DIR" ]; then
	LOCAL_CHECKOUT=1
fi

if [ "$LOCAL_CHECKOUT" -eq 1 ]; then
	if [ "$EXISTING" -eq 1 ] && [ -d "$DIR/.git" ]; then
		log "Atualizando codigo (git pull)..."
		git -C "$DIR" pull --ff-only || warn "git pull falhou; mantendo a copia atual"
	fi
elif [ "$EXISTING" -eq 1 ]; then
	log "Atualizando arquivos para ${VERSION}..."
	download_tarball "$DIR"
else
	download_tarball "$DIR"
fi

FRESH=0
if [ ! -f "$DIR/.env" ]; then
	FRESH=1
	if [ -z "$DOMAIN" ]; then
		DOMAIN=$(prompt 'Dominio do painel (Enter = localhost, sem TLS): ')
	fi
	case "$DOMAIN" in
		*[!A-Za-z0-9.-]*) die "dominio invalido: $DOMAIN" ;;
	esac
	SITE_ADDRESS="${DOMAIN:-:80}"
	ADMIN_PASS=$(rand_pass)
	cat > "$DIR/.env" <<EOF
# Gerado por install.sh em $(date -u +%Y-%m-%dT%H:%M:%SZ)

SITE_ADDRESS=${SITE_ADDRESS}

AGENTE_IA_ADDR=127.0.0.1:8080
AGENTE_IA_PASS=${ADMIN_PASS}
AGENTE_IA_SECRETS_KEY=$(rand_b64 32)
WHATSAPP_HMAC_KEY=$(rand_b64 32)

POSTGRES_USER=notreve_db_admin
POSTGRES_PASSWORD=$(rand_pass)
POSTGRES_DB=agente_ia

APP_DATABASE_USER=agente_app
APP_DATABASE_PASSWORD=$(rand_pass)

RABBITMQ_DEFAULT_USER=notreve
RABBITMQ_DEFAULT_PASS=$(rand_pass)
EOF
	chmod 600 "$DIR/.env"
	log "Configuracao gerada em $DIR/.env"
else
	SITE_ADDRESS=$(env_get SITE_ADDRESS)
	ADMIN_PASS=$(env_get AGENTE_IA_PASS)
fi

PG_USER=$(env_get POSTGRES_USER)
PG_DB=$(env_get POSTGRES_DB)
PG_USER="${PG_USER:-notreve_db_admin}"
PG_DB="${PG_DB:-agente_ia}"

if [ "$EXISTING" -eq 1 ]; then
	mkdir -p "$DIR/backups"
	BACKUP_FILE="$DIR/backups/pg-$(date +%Y%m%d-%H%M%S).sql"
	log "Backup do banco em $BACKUP_FILE ..."
	if (cd "$DIR" && docker compose exec -T postgres pg_dump -U "$PG_USER" "$PG_DB" > "$BACKUP_FILE" 2>/dev/null); then
		log "Backup concluido."
	else
		warn "backup nao pode ser feito agora (seguindo com a atualizacao)"
		rm -f "$BACKUP_FILE"
	fi
fi

cd "$DIR"

log "Validando configuracao..."
docker compose config >/dev/null || die "configuracao invalida (revise $DIR/.env)"

log "Subindo containers (o primeiro build pode demorar)..."
docker compose up -d --build

log "Aguardando o painel ficar saudavel..."
i=0
while :; do
	APP_ID=$(docker compose ps -q app 2>/dev/null || true)
	STATUS=""
	if [ -n "$APP_ID" ]; then
		STATUS=$(docker inspect -f '{{.State.Health.Status}}' "$APP_ID" 2>/dev/null || true)
	fi
	if [ "$STATUS" = "healthy" ]; then
		break
	fi
	i=$((i + 1))
	if [ "$i" -ge "$HEALTH_RETRIES" ]; then
		warn "timeout aguardando o health"
		docker compose logs app --tail 50 || true
		die "o painel nao ficou saudavel a tempo"
	fi
	sleep "$HEALTH_SLEEP"
done

if [ -n "$SITE_ADDRESS" ] && [ "$SITE_ADDRESS" != ":80" ]; then
	log "Registrando o dominio $SITE_ADDRESS ..."
	if docker compose exec -T postgres psql -U "$PG_USER" -d "$PG_DB" -c "UPDATE tenants SET dominio = '$SITE_ADDRESS' WHERE id = 1;" >/dev/null 2>&1; then
		log "Dominio registrado."
	else
		warn "nao foi possivel registrar o dominio agora (ajuste no painel)"
	fi
fi

if [ -f "$DIR/ncc" ]; then
	if ln -sf "$DIR/ncc" /usr/local/bin/ncc 2>/dev/null; then
		chmod 0755 "$DIR/ncc" 2>/dev/null || true
	fi
fi

if [ "$SITE_ADDRESS" = ":80" ] || [ -z "$SITE_ADDRESS" ]; then
	URL="http://localhost"
else
	URL="https://${SITE_ADDRESS}"
fi

printf '\n'
if [ "$EXISTING" -eq 1 ]; then
	printf '%s atualizado com sucesso.\n\n' "$APP_NAME"
else
	printf '%s instalado com sucesso.\n\n' "$APP_NAME"
fi
printf '  Painel:   %s\n' "$URL"
printf '  Usuario:  super_admin\n'
if [ "$FRESH" -eq 1 ]; then
	printf '  Senha:    %s\n' "$ADMIN_PASS"
	printf '\n  Guarde a senha: ela nao sera mostrada novamente.\n'
fi
printf '\n  Proximos passos:\n'
printf '    1. Acesse o painel e ative a verificacao em duas etapas (MFA).\n'
printf '    2. Rode o assistente: provedor de IA -> agente -> canal.\n'
printf '\n  Gerenciar: ncc up | down | logs | update | backup | doctor\n'
printf '  Atualizar: sudo ncc update\n\n'
