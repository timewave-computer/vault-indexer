
# Positions
`/v1/vault/vaultAddress/positions` 

With query args:
`/v1/vault/vaultAddress/positions?ethereum_address=XXX`
`/v1/vault/vaultAddress/positions?ethereum_address=XXX&from=50&order=asc` 

## Optional query params
```json
{
	"ethereum_address": "address to filter by",
	// for pagination
	"from": "from position Id",
	"limit": "number records",
	"order": "asc | desc"
}
```



