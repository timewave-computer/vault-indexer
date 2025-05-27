# Indexer

## Requirements
- connect to an EVM node with websocket in Go
- let me configure what i listen to in a config file. for each contract, i provide contract address, start block, and ABI
- subscribe to Deposit, Withdraw, and Transfer events
- write events to postgres DB
- from events table, create a dependant table implements the transformation required for a Position table and accounts table
- provide an API for accounts and positions per the API spec
- provide a recovery path where if i restart the indexer, load all data from the start block of the contract, and then start recording events once log is processed


## Script sturcture
1. connecting to an evm node in go
2. consuming a config file for what to index (with a sample config file)
3. loading in all events that already occured
4. listening to new events once log is processed
5. writing events to postgres
6. postgres transforms events table to Positions table as the Events table gets written

