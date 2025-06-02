# Indexer

## Architecture
- Main indexer process starts, loads config for what to index and what to connect to, connects to the eth client and database, starts up channels for data transformation after ingestion
- Begins event processing
   1. for each contract and event in the config, process historical events. for each event, save in DB and send to post processing (positions)
   2. for each contract and event in the config, subscribe to event. as they come in, send to processing
- Passes data to post-process after event ingestion
    1. from positions channel: read and process positions


## Requirements
1. connecting to an evm node in go
2. consuming a config file for what to index (with a sample config file)
3. loading in all events that already occured
4. listening to new events once log is processed
5. writing events to postgres
6. postgres transforms events table to Positions table as the Events table gets written


## Prereqs
Indexer:
- go
- supabase installed globally (`npm install supabase --save-dev`)
- docker


## Start Indexer
1. install dependencies
```bash
go mod tidy
```

2. start local DB
start docker
```bash
npx supabase start
npx supabase status

# deploy schema to DB
npx supabase migration up
# update DB types
npx supabase gen types --lang go --local > go-indexer/database/types.go

# to reset
npx supabase db reset
## remember to re-generate types

# reset system
npx supabase stop
docker network prune

```

3. set config in `config/indexer-config.dev.toml`
- add contracts info + abis
- check supabase local studio (http://127.0.0.1:54323) for anon key and connection string

3. run server
```bash
# For development (default)
./start-indexer.sh

# For production
./start-indexer.sh prod
```

## Testing
```bash
go test ./...  

go test ./go-indexer/indexer -v
```

# API server
Simple nextjs app to provide an API for the indexer database

## Prereqs

- node.js
- npm

## Start API server
1. install dependencies
```bash
npm install
```

2. start server
```bash
npm run dev
```