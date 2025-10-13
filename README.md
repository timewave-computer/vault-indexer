# ERC4626 Ethereum Vault Indexer and API Gateway
This project demonstrates the design and implementation of a production-grade Ethereum indexer and API gateway, written entirely in Go.
It ingests on-chain events from an ERC-4626 vault, processes and normalizes them into a Supabase PostgreSQL database, and exposes a public REST API with automatically generated documentation.
The system is built around a concurrent, multi-threaded architecture designed for data integrity, fault tolerance, and maintainable scaling — with live and historical event ingestion, chain reorganization handling, and finalized block tracking.
Deployed on a Linux server and managed via systemd, the project demonstrates a full end-to-end data pipeline: from blockchain ingestion to queryable structured data.

## Features
- ***Ethereum Indexer in Go***: Listens to Deposit, WithdrawRequested, and Transfer events from an ERC-4626 vault contract
- ***Supabase Integration***: Stores normalized event data in a hosted Postgres database
- ***API Gateway***: Exposes REST endpoints (via Next.js) for querying indexed data
- ***Auto-Generated Docs***: OpenAPI documentation generated automatically from Next.js endpoints, available at `<API_URL>/docs`
- ***Resilient Architecture***: Includes queue-based event processing and reorg verification 
- ***Production-Ready Deployment***: Can run as a Linux systemd service for reliability and observability

## Architecture
![Architecture](/docs/architecture.png)
The Vault Indexer is designed as a concurrent Go service that ingests, processes, and serves Ethereum vault events in real time.

It operates through three coordinated threads — each responsible for a specific stage of the indexing pipeline — and exposes the processed data through an API gateway backed by Supabase Postgres.

### Main Indexer Thread
- Connects to an Ethereum node via JSON-RPC and subscribes to the vault’s on-chain events.
- Handles both historical backfill and live streaming of events such as Deposit, WithdrawRequested, and Transfer.
- Writes all captured events into an in-memory event queue, ensuring they’re processed in chronological order.
- Acts as the entry point for all blockchain data entering the system.

### Event Processing Queue
- Serves as a buffer between event ingestion and downstream processing.
- Maintains ordered delivery of events to prevent race conditions and ensure deterministic indexing.
- Enables the system to handle bursts of incoming events without blocking network I/O.

### Data Processor Thread
- Continuously fetches newly queued events and transforms them into structured index records suitable for querying.
- Normalizes Ethereum event data (e.g., addresses, amounts, block metadata) into relational database entries.
- Persists these records into the Postgres database, providing a clean and queryable representation of vault state over time.

### Finality Processor Thread
- Monitors blockchain finality and safety levels to maintain data integrity.
- Detects potential chain reorganizations (reorgs) by verifying block hashes and canonicality.
- Triggers cleanup and rollback operations if non-finalized data becomes invalid.
- Updates internal tables to reflect finalized blocks and ensures all indexed data corresponds to the canonical Ethereum chain.

### Postgres Database (Supabase)
- Acts as the central data store for both raw events and processed indices.
- Enables efficient queries over user positions, vault balances, and event histories.
- Managed through Supabase, which provides hosted PostgreSQL, authentication, and built-in REST access.

### Next.js API Gateway
- Provides a RESTful API layer for external clients, dashboards, or analytics tools.
- Offers auto-generated documentation (Swagger / OpenAPI) for ease of exploration.
- Fetches data directly from the Postgres database, exposing endpoints for querying vault events, user histories, and position summaries.
- Enables fast, queryable access to indexed data by contract address or user wallet.


## Getting Started
### 1. Prerequisites
- indexer
    - go
    - ethereum node (HTTP endpoint)
- api gateway
    - npm
    - node.js
- local postgres development
    - supabase installed globally (`npm install supabase --save-dev`)
    - docker

### 2. Supply environment
Copy `.env.example` and create `.env.local`. You can also create an `.env.prod` and run the process with an environment arg. The default environment is dev.

### 3. Configure vault addresses to index at `go-indexer/config/config.yaml`.

### 4. Start Database
- start docker
- start database
```bash
npx supabase start # will deploy existing schema
npx supabase status # view the available endpoints
```

### 5. Start indexer
```bash
go mod tidy # installs Go dependencies
./start-indexer.sh  # Start server in development (default)
./start-indexer.sh prod # example with .env.prod file
```

### 6. Start API gateway / docs
```bash
npm install
npm run dev
```
Swagger docs will be available at `/docs`

## Working with Supabase
Here are some useful commands for working with the database.

### Deploy schema change 
```bash
npx suapbase migration new <name>
npx supabase migration up
```

### Generate types
```bash
# for indexer
npx supabase gen types --lang go --local > go-indexer/database/types.go

# for api server
npx supabase gen types --lang typescript --local > app/types/database.ts

```

### Clear database and apply migrationx
```bash
npx supabase db reset
## remember to re-generate types
```

### Restart system
```bash
npx supabase stop --no-backup ## drops DB

docker stop $(docker ps -a -q)
docker rm $(docker ps -a -q)
docker network prune

docker volume ls
docker volume rm <volume name>
```

## Deployment
### Indexer
The indexer is meant to run as a constant-uptime script on a remote server. It is safe to stop and restart, and will backfill the data on each restart.

This process was previously deployed on Linux servers, run with systemd commands which were configured via Nix. 

Persistent errors will cause the process to stop running. If you plan to deploy this yourself, make sure to add a configuration which will restart the process with a cooldown if it stops.

## Running with nix
```bash
nix run .#indexer
```

## Testing
```bash
go test ./...  
go test ./go-indexer/indexer -v
```


## Health checks
The main thread spins up a lightweight HTTP server available for health checks.
```bash
curl http://localhost:8080/health
```

## Data Testing
Manaul check which can be run to load all historical data locally, and compare that production matches the historical load.

1. Start with fresh local DB
`npx supabase db reset`

2. Load in historical set of data 
`./start-indexer.sh`

3. Define connection params for test
- populate `go-indexer/scripts/.env.scripts` with local and remote DB urls, and eth websocket RPC url

4. Run comparison script
- `cd go-indexer/scripts` *MUST* do this
- run comparison script `go run data-check.go`