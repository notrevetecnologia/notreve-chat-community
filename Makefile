SHELL := /bin/sh

.PHONY: help build vet test frontend gate run migrate up down logs install

help:
	@printf 'Notreve Chat Community - alvos disponiveis:\n'
	@printf '  make build      Compila os binarios Go\n'
	@printf '  make vet        Roda go vet\n'
	@printf '  make test       Roda os testes Go\n'
	@printf '  make frontend   Compila o frontend React\n'
	@printf '  make gate       build + vet + test + frontend\n'
	@printf '  make run        Sobe o app local (precisa de Postgres e .env)\n'
	@printf '  make migrate    Aplica as migrations\n'
	@printf '  make up         Sobe o stack via docker compose\n'
	@printf '  make down       Derruba o stack\n'
	@printf '  make logs       Logs do app\n'
	@printf '  make install    Instala/atualiza via ./install.sh\n'

build:
	go build -buildvcs=false ./...

vet:
	go vet ./...

test:
	go test ./...

frontend:
	cd web/react && npm ci && npm run build

gate: build vet test frontend

run:
	go run .

migrate:
	go run ./cmd/migrate

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200 app

install:
	./install.sh
