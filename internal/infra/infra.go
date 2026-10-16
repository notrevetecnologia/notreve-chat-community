package infra

import (
	"context"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

// Infra conecta serviços opcionais (Redis/RabbitMQ). Quando não configurados
// (ou indisponíveis), o sistema segue funcionando sem eles.
type Infra struct {
	Redis    *redis.Client
	Rabbit   *amqp.Channel
	rabbitConn *amqp.Connection
}

func Connect(ctx context.Context, redisURL, rabbitURL string) *Infra {
	inf := &Infra{}

	if redisURL != "" {
		if opt, err := redis.ParseURL(redisURL); err == nil {
			c := redis.NewClient(opt)
			if err := c.Ping(ctx).Err(); err == nil {
				inf.Redis = c
				log.Printf("Redis conectado")
			} else {
				log.Printf("Redis indisponível (continuando sem): %v", err)
			}
		}
	}

	if rabbitURL != "" {
		conn, err := amqp.Dial(rabbitURL)
		if err != nil {
			log.Printf("RabbitMQ indisponível (continuando sem): %v", err)
		} else {
			ch, err := conn.Channel()
			if err == nil {
				inf.Rabbit = ch
				inf.rabbitConn = conn
				_, _ = ch.QueueDeclare("notreve_agente_ia", true, false, false, false, nil)
				log.Printf("RabbitMQ conectado")
			}
		}
	}
	return inf
}

// PublishEvent publica um evento na fila de forma assíncrona e segura.
// Não bloqueia e ignora se o RabbitMQ não estiver disponível.
func (inf *Infra) PublishEvent(queue, body string) {
	if inf.Rabbit == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("rabbit publish recover: %v", r)
			}
		}()
		_ = inf.Rabbit.Publish("", queue, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
		})
	}()
}

func (inf *Infra) Close() {
	if inf.Redis != nil {
		_ = inf.Redis.Close()
	}
	if inf.rabbitConn != nil {
		_ = inf.rabbitConn.Close()
	}
}
