# Indexer


## Requirements
1. connecting to an evm node in go
2. consuming a config file for what to index (with a sample config file)
3. loading in all events that already occured
4. listening to new events once log is processed
5. writing events to postgres
6. postgres transforms events table to Positions table as the Events table gets written

## Developing
1. install dependencies
```bash
go mody tidy
```