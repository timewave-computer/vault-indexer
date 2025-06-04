# Indexer + API app
The project has 3 parts
- Postgres DB - schema defined in `supabase` hosted by supabase, uses some of their tools
- Indexer "ETL" go code in `go-indexer` - long-running server that
    - connects to an ethereum node
    - reads events for specific contracts (configured in `indexer-config`)
    - write events to DB
    - transforms events into data required for the app (accounts, positions, etc)
- Next JS API app in `app` - easy way to host the API. reads from the same schema. Deployed on vercel.

## Database
### Prerequisites
- npm
- docker
- supabase installed globally (`npm install supabase --save-dev`)

### Start locally
start docker
```bash
npx supabase start
npx supabase status
```

### Deploy schema change 
```bash
npx supabase migration up
```

### Generate types
```bash
# for indexer
npx supabase gen types --lang go --local > go-indexer/database/types.go

# for api server
npx supabase gen types --lang typescript --local > app/types/database.ts

```

### Clear database and apply migration
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


```

## Indexer

### Prereqs
- go
- docker

### Local development
1. install dependencies
```bash
go mod tidy
```

2. set config in `indexer-config/config.dev.toml`
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

2. copy `.env.example`
- SUPABASE_URL: http url for postgres db
- SUPABASE_ANON_KEY: read-only key
- API_KEY: random string to give access to API

3. start server
```bash
npm run dev
```
Visit the api at `localhost:3000/v1`

## Updating API schema
The automated process fails in deployment, so this is a temporary approach.

- visit `/api-gen` locally and copy static file
- paste in `public/openapi.json`


