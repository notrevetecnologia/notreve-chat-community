FROM node:25-alpine AS react-build
WORKDIR /react
COPY web/react/package*.json ./
RUN npm ci --no-audit --no-fund
COPY web/react/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /notreve-chat-community .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /notreve-chat-community-migrate ./cmd/migrate

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /notreve-chat-community .
COPY --from=build /notreve-chat-community-migrate .
COPY web ./web
COPY --from=react-build /react/dist ./web/react/dist
COPY migrations ./migrations
ENV AGENTE_IA_WEBROOT=/app/web \
    AGENTE_IA_MIGRATIONS=/app/migrations
EXPOSE 8080
ENTRYPOINT ["/app/notreve-chat-community"]
