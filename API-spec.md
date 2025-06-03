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

## Response Type
```typescript
{
  data: Array<{
    id: number;              // position_index_id
    amount_shares: number;   // amount of shares in the position
    position_start_height: number;  // block height when position was opened
    position_end_height: number;    // block height when position was closed (if applicable)
    ethereum_address: string;       // address of the position owner
    created_at: string;            // timestamp of position creation
  }>
}
```

## Error Response
```typescript
{
  error: string;  // Error message
}
```

## Notes
- Returns HTTP 400 for invalid requests (e.g., invalid ethereum address)
- Results are ordered by position_index_id
- Pagination is supported using 'from' and 'limit' parameters
- Filtering by ethereum_address is optional

# Withdraw Requests
`/v1/vault/vaultAddress/withdrawRequest/ethereumAddress`

## Optional query params
```json
{
  // for pagination
  "from": "from withdraw_id",
  "limit": "number of records",
  "order": "asc | desc"
}
```

## Response Type
```typescript
{
  data: Array<{
    id: number;              // withdraw_id
    amount: number;          // amount of the withdraw request
    created_at: string;      // timestamp of withdraw request creation
    neutron_address: string; // neutron address associated with the request
  }>
}
```

## Error Response
```typescript
{
  error: string;  // Error message
}
```

## Notes
- Returns HTTP 400 for invalid requests (e.g., invalid vault address or ethereum address)
- Results are ordered by withdraw_id
- Pagination is supported using 'from' and 'limit' parameters
- Both vaultAddress and ethereumAddress are required path parameters



