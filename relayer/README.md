# Veilo Relayer

Production-grade monolith relayer backend for the Veilo Privacy Pool protocol on Solana.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Layer (Gin)                          │
│  /api/v1/health | /api/v1/relay | /api/v1/transactions         │
├─────────────────────────────────────────────────────────────────┤
│                     Use Case Layer                              │
│  RelayUseCase │ TransactionUseCase │ PoolUseCase                │
├─────────────────────────────────────────────────────────────────┤
│                   Domain Layer (Entities)                       │
│  Transaction │ Pool │ Relayer │ Domain Events                   │
├─────────────────────────────────────────────────────────────────┤
│                Infrastructure Layer                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │ Solana   │  │PostgreSQL│  │  Redis   │  │Event Bus │       │
│  │ Client   │  │    DB    │  │  Cache   │  │  (NATS)  │       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

## Design Patterns

- **Clean Architecture** - Separation of concerns with domain, use case, infrastructure, and interface layers
- **Repository Pattern** - Abstract data access behind interfaces
- **Dependency Injection** - Constructor-based DI for all components
- **CQRS** - Command Query Responsibility Segregation for read/write operations
- **Event-Driven** - Asynchronous event publishing for transaction lifecycle
- **Circuit Breaker** - Fault tolerance for external service calls

## Features

- **Transaction Relay** - Submit and monitor Solana transactions
- **Fee Management** - Configurable fee structure with BPS
- **Pool Monitoring** - Real-time pool health monitoring
- **Statistics** - Transaction volume, success rates, fee analytics
- **WebSocket** - Real-time transaction status updates
- **Health Checks** - Comprehensive system health endpoint

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 14+
- Redis 7+
- Solana CLI (optional)

### Installation

```bash
# Clone repository
git clone https://github.com/popololo229099-svg/veilo-relayer.git
cd veilo-relayer

# Install dependencies
go mod download

# Set up database
createdb veilo_relayer

# Configure
cp configs/config.yaml.example configs/config.yaml
# Edit configs/config.yaml

# Run
go run cmd/server/main.go
```

### Docker

```bash
docker-compose up -d
```

## API Endpoints

### Health Check

```bash
GET /api/v1/health

Response:
{
  "status": "healthy",
  "solBalance": 1000000000,
  "slot": 432860998,
  "version": "1.0.0",
  "uptime": 1692278400
}
```

### Relay Transaction

```bash
POST /api/v1/relay

Request:
{
  "type": "swap",
  "pool": "GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU",
  "proof": "...",
  "publicInputs": ["..."],
  "swapParams": {
    "amountIn": 1000000000,
    "minAmountOut": 990000000,
    "fee": 5000000,
    "recipient": "...",
    "relayer": "...",
    "swapDataHash": "..."
  }
}

Response:
{
  "transactionId": "...",
  "signature": "...",
  "status": "submitted",
  "fee": 5000000
}
```

### Get Transactions

```bash
GET /api/v1/transactions?limit=50

Response:
{
  "transactions": [...],
  "count": 50
}
```

### Get Statistics

```bash
GET /api/v1/stats

Response:
{
  "totalTransactions": 1234,
  "successfulTxs": 1200,
  "failedTxs": 34,
  "totalVolume": 1000000000000,
  "totalFees": 5000000000,
  "successRate": 97.24
}
```

## Configuration

```yaml
server:
  host: "0.0.0.0"
  port: "8080"

solana:
  rpc: "https://api.mainnet-beta.solana.com"
  ws: "wss://api.mainnet-beta.solana.com"

database:
  host: "localhost"
  port: 5432
  name: "veilo_relayer"
  user: "postgres"
  password: "postgres"

redis:
  host: "localhost"
  port: 6379

relayer:
  private_key: ""  # Set via RELAYER_PRIVATE_KEY env var
  fee_bps: 50      # 0.5% fee
  min_fee: 1000000  # 0.001 SOL minimum fee
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `RELAYER_PRIVATE_KEY` | Solana private key for signing | - |
| `SOLANA_RPC` | Solana RPC endpoint | `https://api.mainnet-beta.solana.com` |
| `DATABASE_HOST` | PostgreSQL host | `localhost` |
| `DATABASE_PORT` | PostgreSQL port | `5432` |
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |

## Project Structure

```
relayer/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── configs/
│   └── config.yaml              # Configuration
├── internal/
│   ├── domain/
│   │   └── entities.go          # Domain entities & interfaces
│   ├── usecase/
│   │   └── relay.go             # Business logic
│   ├── infrastructure/
│   │   ├── solana/
│   │   │   └── client.go        # Solana client
│   │   ├── database/
│   │   │   └── postgres.go      # PostgreSQL repositories
│   │   └── cache/
│   │       └── redis.go         # Redis cache
│   └── interfaces/
│       ├── api/
│       │   └── handlers.go      # HTTP handlers
│       └── worker/
│           └── worker.go        # Background workers
├── go.mod
├── go.sum
└── README.md
```

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o bin/relayer cmd/server/main.go
```

### Linting

```bash
golangci-lint run
```

## License

MIT
