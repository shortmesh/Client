package rabbitmq

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"maunium.net/go/mautrix"
)

func Sender(client *mautrix.Client, message string, exchange string) error {
	conn, ch, q, err := start(client, exchange)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	defer ch.Close()
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx,
		exchange, // exchange
		q.Name,   // routing key
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message), // TODO: set TTL
		},
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	slog.Debug("RMQ sent", "message", message)
	return nil
}
