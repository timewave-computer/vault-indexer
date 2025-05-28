# Indexer


## Requirements
1. connecting to an evm node in go
2. consuming a config file for what to index (with a sample config file)
3. loading in all events that already occured
4. listening to new events once log is processed
5. writing events to postgres
6. postgres transforms events table to Positions table as the Events table gets written


## Prereqs
- go
- supabase installed globally (`npm install supabase --save-dev`)
- docker

## Developing
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

# to reset
npx supabase db reset

# reset system
npx supabase stop
docker network prune

```

3. set config in `config/config.dev.toml`
- add contracts info + abis
- check supabase local studio (http://127.0.0.1:54323) for anon key and connection string

3. run server
```bash
# For development (default)
./start.sh

# For production
./start.sh prod
```