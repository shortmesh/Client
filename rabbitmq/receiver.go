package rabbitmq

import (
	"log"
	"log/slog"
	"runtime/debug"

	amqp "github.com/rabbitmq/amqp091-go"
)

func Receiver() error {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"hello", // name
		false,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	slog.Debug("RabbitMQ waiting for messages")
	<-forever
	return nil
}
