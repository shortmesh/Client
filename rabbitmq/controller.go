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

func start(client *mautrix.Client, exchange string) (*amqp.Connection, *amqp.Channel, *amqp.Queue, error) {
	slog.Debug("RabbitMQ starting", "username", client.UserID.String())
	conn, err := getConnection()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, nil, err
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
		return nil, nil, nil, err
	}

	q, err := ch.QueueDeclare(
		client.UserID.Localpart(), // name
		false,                     // durable
		false,                     // delete when unused
		false,                     // exclusive
		false,                     // no-wait
		nil,                       // arguments
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, nil, err
	}

	return conn, ch, &q, nil
}
