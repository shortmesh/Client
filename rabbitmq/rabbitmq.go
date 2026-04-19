package rabbitmq

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shortmesh/core/configs"
	"maunium.net/go/mautrix"
)

func getConnection() (*amqp.Connection, error) {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	username := conf.RabbitMQ.Username
	password := conf.RabbitMQ.Password
	host := conf.RabbitMQ.Host
	port := conf.RabbitMQ.Port
	protocol := "amqp"
	if conf.RabbitMQ.IsTLs {
		protocol = "amqps"
	}

	conn, err := amqp.Dial(fmt.Sprintf("%s://%s:%s@%s:%d/", protocol, username, password, host, port))
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return conn, nil
}

func DeleteQueue(client *mautrix.Client, queueName string) error {
	conn, err := getConnection()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	_, err = ch.QueueDelete(
		queueName,
		false, // ifUnused
		false, // ifEmpty
		false, // noWait
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}

func start(
	client *mautrix.Client,
	exchange,
	bindingKey,
	queueName string,
) (*amqp.Connection, *amqp.Channel, error) {
	slog.Debug("RabbitMQ starting", "username", client.UserID.String())
	conn, err := getConnection()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, err
	}

	err = ch.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, err
	}

	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, err
	}

	err = ch.QueueBind(
		q.Name,     // queue name
		bindingKey, // routing key
		exchange,   // exchange name
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, err
	}

	return conn, ch, nil
}
