# ShortMesh Core - Architecture Overview

## Project Summary

ShortMesh Core is a Matrix bridge system that provides authentication, device management, and messaging capabilities across multiple platforms (WhatsApp, Signal) through a Matrix homeserver. It's a Go-based backend system that uses SQLite databases for persistence and RabbitMQ for message queuing.

## Tech Stack

### Backend
- **Language**: Go (1.24.3)
- **Framework**: Gin Web Framework for REST API
- **Database**: SQLite (via mattn/go-sqlite3)
- **Message Queue**: RabbitMQ (amqp091-go)
- **Client Library**: mautrix (Matrix client SDK)

### Python Component
- **Language**: Python (3.14)
- **Dependencies**: certifi, charset-normalizer, idna, requests, urllib3
- **Purpose**: Python client functionality (see `clients.py`)

## Directory Structure

```
core/
├── main.go                 # Main entry point and REST API server
├── go.mod                  # Go module dependencies
├── requirements.txt        # Python dependencies
├── conf.yaml.example       # Configuration template
│
├── apis/                   # API handlers
│   ├── login.go            # Login endpoint handlers
│   ├── store.go            # User credential storage
│   ├── devices.go          # Device management endpoints
│   └── messages.go         # Message sending endpoints
│
├── bridges/                # Bridge functionality for external services
│   ├── bridges.go          # Core bridge logic and message processing
│   └── sessions-db.go      # Session management
│
├── cmd/                    # Command-line utilities
│   ├── matrix-client.go    # Matrix client initialization
│   └── controllers.go      # Controller logic
│
├── configs/                # Configuration management
│   └── configs.go          # Config loading and validation
│
├── users/                  # User management
│   ├── users.go            # User CRUD operations
│   └── users-db.go         # User database operations
│
├── devices/                # Device management
│   └── devices.go          # Device CRUD operations
│
├── rooms/                  # Room management
│   └── rooms.go            # Room-related operations
│
├── rabbitmq/               # RabbitMQ integration
│   └── rabbitmq.go         # Message queue consumer and publisher
│
├── utils/                  # Utility functions
│   └── helpers.go          # Helper utilities
│
├── db/                     # SQLite databases
│   ├── clients.db          # Client/user registry
│   └── <username>.db       # User-specific databases
│
└── clients.py              # Python client application
```

## Core Components

### 1. REST API (main.go)
The main entry point that launches multiple concurrent routines:

- **API Server**: Gin router serving REST endpoints
  - `POST /login` - User authentication
  - `POST /store` - Store user credentials
  - `GET /devices` - List user devices
  - `POST /devices` - Add new device
  - `DELETE /devices/:deviceId` - Remove device
  - `POST /devices/:deviceId/message` - Send message
  - `GET /docs/*any` - Swagger API documentation

- **SyncUsers**: Background goroutine for user synchronization
- **RabbitMQReceiver**: Message consumer from RabbitMQ queue
- **CORS**: Configured for cross-origin requests

### 2. Authentication & User Management (users/)
Handles user authentication and credential storage:

**User Types** (users/users.go:14-21):
- `User` - Regular Matrix user
- `BridgeBot` - Bridge automation bot
- `Device` - Connected external device
- `Contact` - Matrix contact

**Key Functions**:
- `Save()` - Store user credentials (access token, pickle key, etc.)
- `FetchUser()` - Retrieve user from database
- `FetchAllUsers()` - List all registered users
- `GetTypeUser()` - Determine user type based on ID

### 3. Device Management (devices/)
Manages external device connections:

- `Save()` - Store device credentials
- `GetDevices()` - List user's devices
- `IsDevice()` - Check if user ID belongs to a device

### 4. Bridge System (bridges/)
Connects Matrix with external services (WhatsApp, Signal):

**Bridge Configuration** (conf.yaml):
```yaml
bridges:
  - name: wa
    botname: "@whatsappbot:matrix.sherlockwisdom.com"
    username_template: "whatsapp_{{.}}"
    display_username_template: "{{.}} (WA)"
    cmd:
      login: "login qr"
      list-logins: "* `%s` (+%s) - `CONNECTED`"
      devices: "list-logins"
      logout: "logout %s"
```

**Key Functions**:
- `LookupBridgeByName()` - Find bridge by name
- `LookupBridgeByRoomId()` - Find bridge by room ID
- `JoinManagementRooms()` - Join bridge management room
- `AddDevice()` - Add new device via bot command
- `RemoveDevice()` - Remove device via bot command
- `processIncomingMessages()` - Handle incoming messages from bridge bot

**Message Processing**:
- Parses bot responses for success/failure
- Extracts device IDs from messages
- Saves devices automatically

### 5. Database Structure

**SQLite Databases**:
- `clients.db` - Global registry of all Matrix clients
- `<username>.db` - User-specific data (users, contacts, devices)

**Table Schema** (inferred from code):
- Users table: username, access_token, device_id, recovery_key, pickle_key
- Devices table: device_id, bridge_name
- Contacts table: contact_id, room_id
- Rooms table: room_id, bridge_name

### 6. RabbitMQ Integration (rabbitmq/)
Message queue for async communication:

- **Queue**: "hello" (non-durable)
- **Consumer**: Receives messages from queue
- **Publisher**: Sends messages to queue (example implementation)
- **Connection**: `amqp://guest:guest@localhost:5672/`

### 7. API Endpoints (apis/)

**Login (apis/login.go)**:
- Authenticates users against Matrix homeserver
- Handles device login and QR code scanning

**Store (apis/store.go)**:
- Stores user credentials securely
- Handles pickle key encryption/decryption

**Devices (apis/devices.go)**:
- GET /devices - Retrieve user's registered devices
- POST /devices - Add new device
- DELETE /devices/:deviceId - Remove device

**Messages (apis/messages.go)**:
- POST /devices/:deviceId/message - Send messages to devices

## Data Flow

### Authentication Flow
1. User calls POST /login with credentials
2. System creates Matrix client
3. Performs authentication
4. Stores credentials in database
5. Returns success response

### Device Management Flow
1. User calls POST /devices
2. System identifies bridge to use
3. Sends command to bridge bot via Matrix room
4. Bridge bot processes external service login
5. Bot returns success/failure message
6. System parses response and saves device ID

### Message Flow
1. User calls POST /devices/:deviceId/message
2. System retrieves device information
3. Routes message to appropriate bridge
4. Bridge bot sends message to external service
5. Returns delivery status

## Configuration

**Main Configuration** (conf.yaml):
- Server host/port and TLS settings
- Bridge configurations (WhatsApp, Signal, etc.)
- Home server URL and domain
- Keystore file path

**Environment Requirements**:
- RabbitMQ running on localhost:5672
- SQLite database files in `db/` directory
- TLS certificates (optional) for HTTPS

## Security Considerations

1. **Credential Storage**: User credentials stored with pickle keys for encryption
2. **CORS**: Configured for all origins (should be restricted in production)
3. **Database**: SQLite files should have proper permissions
4. **API Authentication**: Requires access token validation

## Dependencies

### Go
- gin-gonic/gin - Web framework
- mattn/go-sqlite3 - SQLite driver
- rabbitmq/amqp091-go - RabbitMQ client
- swaggo/* - Swagger documentation
- gopkg.in/yaml.v3 - YAML parsing
- mautrix - Matrix client SDK

### Python
- requests - HTTP requests
- urllib3, certifi, charset-normalizer, idna - Network utilities

## Running the Application

1. Copy configuration: `cp conf.yaml.example conf.yaml`
2. Install dependencies: `go mod tidy`
3. Start services: `go run .`

## API Documentation

Access Swagger documentation at: `http://host:port/docs` (configure in conf.yaml)

## Future Enhancements

Based on code analysis, potential improvements:
- PostgreSQL support (currently using SQLite)
- Better error handling and logging
- Token rotation and refresh mechanism
- Rate limiting for API endpoints
- More robust bridge detection and error handling