# Improving Indexer Robustness

- on startup, fetch current block
- for each contract and event:
   - fetch last processed block # for contract+event name
   - start subscription and writing events immediately
   - backfill [lastProcessedBlock,currentBlock]
- after all backfilling + subscribing is complete, start transformer
- if there is connectivity error with WS, close
- if there is errors with PG or supabase, retry and then close
- outside supervisor should restart the process 
