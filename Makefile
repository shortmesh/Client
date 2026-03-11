all: build test

setup:
	@echo "Setting up development environment..."
	@if [ ! -f conf.yaml ]; then \
		echo "Creating conf.yaml from conf.yaml.example..."; \
		cp conf.yaml.example conf.yaml; \
	fi
	@if ! grep -q "^db_key: [A-Fa-f0-9]\{64,\}" conf.yaml 2>/dev/null; then \
		echo "Generating db_key..."; \
		DB_KEY=$$(openssl rand -hex 32); \
		sed -i.bak "s|^db_key:.*|db_key: \"$$DB_KEY\"|" conf.yaml && rm -f conf.yaml.bak; \
	else \
		echo "db_key already set"; \
	fi
	@echo "Installing dependencies..."
	@go mod tidy
	@echo "Setup complete! Update conf.yaml with your homeserver details, then run 'make run' to start."

build: docs
	@echo "Building binary..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/client .

run: docs
	@go run .

docs:
	@echo "Generating Swagger documentation..."
	@which swag > /dev/null || (echo "Error: swag is not installed." && echo "Install it with: go install github.com/swaggo/swag/cmd/swag@latest" && exit 1)
	@swag init

test:
	@echo "Testing..."
	@go test ./... -v

clean:
	@echo "Cleaning..."
	@rm -rf bin

setup-systemd:
	@echo "Setting up systemd service..."
	@if [ "$$(id -u)" -ne 0 ]; then \
		echo "Error: This target must be run as root (use sudo make setup-systemd)"; \
		exit 1; \
	fi
	@if ! id matrix-client >/dev/null 2>&1; then \
		echo "Creating matrix-client user..."; \
		useradd --system --no-create-home --shell /usr/sbin/nologin matrix-client; \
	else \
		echo "User matrix-client already exists"; \
	fi
	@echo "Creating application directory..."
	@mkdir -p /opt/matrix-client
	@if [ ! -f /opt/matrix-client/conf.yaml ]; then \
		echo "Creating conf.yaml file from conf.yaml.example..."; \
		cp conf.yaml.example /opt/matrix-client/conf.yaml; \
		echo "Generating db_key..."; \
		DB_KEY=$$(openssl rand -hex 32); \
		sed -i "s|^db_key:.*|db_key: \"$$DB_KEY\"|" /opt/matrix-client/conf.yaml; \
		echo "WARNING: Review and update /opt/matrix-client/conf.yaml with:"; \
		echo "  - homeserver"; \
		echo "  - homeserver_domain"; \
		echo "  - mas_client_id"; \
		echo "  - mas_client_secret"; \
		echo "  - RabbitMQ configuration"; \
		echo "  - Bridge configurations"; \
	else \
		echo "conf.yaml file already exists at /opt/matrix-client/conf.yaml"; \
	fi
	@echo "Creating data directory..."
	@mkdir -p /opt/matrix-client/data
	@echo "Creating downloads directory..."
	@mkdir -p /opt/matrix-client/downloads
	@echo "Creating cache directories..."
	@mkdir -p /opt/matrix-client/.cache/go-build /opt/matrix-client/.cache/go-mod
	@echo "Setting permissions..."
	@chown -R matrix-client:matrix-client /opt/matrix-client
	@chmod 600 /opt/matrix-client/conf.yaml
	@echo "Installing systemd service file..."
	@cp matrix-client.service /etc/systemd/system/
	@systemctl daemon-reload
	@echo "Enabling service..."
	@systemctl enable matrix-client
	@echo "Setup complete! Use 'systemctl start matrix-client' to start the service."

.PHONY: all setup build run docs test clean setup-systemd
