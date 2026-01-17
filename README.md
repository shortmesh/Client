# ShortMesh core

## Contents

- Installations

- [Schemas](https://github.com/shortmesh/Core/blob/master/README.md#schemas)

- [Notes](https://github.com/shortmesh/Core/blob/master/README.md#notes)

    - [Postgress issues](https://github.com/shortmesh/Core/blob/master/README.md#postgres-issues)
    
    - [Synapse](https://github.com/shortmesh/Core/blob/master/README.md#snaypse)
    
    - [MAS](https://github.com/shortmesh/Core/blob/master/README.md#mas)

## Installation

MacOS (M-series)
```bash
brew install libolm
export LIBRARY_PATH="/opt/homebrew/lib:$LIBRARY_PATH"
export CPATH="/opt/homebrew/include:$CPATH"
```
 
## Schemas
```
Users
____
user_name string ""

access_token string ""

```

```
Rooms
────
room_id string ""

room_type enum direct | management | group

is_encrypted boolean # True is bridge is encrypted

bridge_name string # The actual name is exposed to the bridge; mostly used to ID groups 

```

## Notes
### Postgres issues
- [Cannot connect to Postgres database](https://github.com/matrix-org/synapse/issues/2780#issuecomment-855285811)

### Snaypse
```nginx
server {
    listen 443 ssl http2;
    server_name matrix.example.com;

    ssl_certificate /etc/letsencrypt/live/matrix.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/matrix.example.com/privkey.pem;

    client_max_body_size 50M;

    # MAS-backed client auth routes
    location ~ ^/_matrix/client/(v3|v1)/(login|logout|refresh|auth_metadata|capabilities) {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Synapse endpoints
    location ~ ^(/_matrix|/_synapse/client|/_synapse/mas) {
        proxy_pass http://127.0.0.1:8008;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # .well-known
    location /.well-known/matrix/ {
        alias /var/www/matrix/.well-known/matrix/;
        default_type application/json;
        add_header Access-Control-Allow-Origin *;
    }
}
```

***.well-known/client***
```json
{
  "m.homeserver": {
    "base_url": "https://matrix.example.com"
  },
  "org.matrix.msc2965.authentication": {
    "issuer": "https://auth.example.com/",
    "account": "https://auth.example.com/account/"
  }
}
```
### MAS
**configuration**
- [Setup](https://willlewis.co.uk/blog/posts/stronger-matrix-auth-mas-synapse-docker-compose/)

**config.yaml**
```yaml
http:
  listeners:
  - name: web
    resources:
    - name: discovery
    - name: human
    - name: oauth
    - name: compat
    - name: graphql
    - name: assets
    binds:
      # - address: '[::]:8080'
      - host: 0.0.0.0
        port: 8080
    proxy_protocol: false
  - name: internal
    resources:
    - name: health
    binds:
    - host: localhost
      port: 8081
    proxy_protocol: false
  trusted_proxies:
  - 192.168.0.0/16
  - 172.16.0.0/12
  - 10.0.0.0/10
  - 127.0.0.1/8
  - fd00::/8
  - ::1/128
  public_base: https://auth.example.com/
  issuer: https://auth.example.com/
...
matrix:
  kind: synapse
  homeserver: matrix.example.com
  endpoint: https://matrix.sherlockwisdom.com/
  secret: R8PHHknWdVHIsIgUODRuFcN9XYINtrNO
account:
  password_registration_enabled: true
  password_recovery_enabled: true
  account_deactivation_allowed: true
  login_with_email_allowed: true
```
