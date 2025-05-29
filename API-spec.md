# Accounts

`/vaultAddress/accounts` 

- to fetch all accounts (evm addresses) which have an associated position
- paginated
- returns list of EVM addresses

### **Output**

```jsx
{
	data: [
		{
			evm_address:string
		}
	],
	pagination: 
	{
		// pagination data (cursor & page info. struct TBD)
	}
}
```

# Positions

`/vaultAddress/positions?address=<evm_address>` 

### Data Model

```jsx
 Position {
	id: number // incrementing counter
	ethereum_address: string
	neutron_address: string | null // neutron address recorded for withdraw
	position_start_height: number // block number position was created
	position_end_height: number | null // block number position was modified -1
	amount_shares: number // amount_shares of asset store
	
	// 'nice to haves' for quality of life
	isTerminated: bool  // position for ethereum address becomes 0

	// TODO:
	isDeposit: bool
	isTransfer:bool

}
```

`Position` start block and amount_shares will be immutable. Any time a position change is captured for an EVM address (deposit, withdraw, transfer), the latest position will be ‘closed’ at `block_height -1,` and a new `Position` will be opened at `block_height`. 

### Input

```jsx
evm_address: string (optional) // option to filter positions by a user address
```

### Output

```jsx
{
	data: [Position],
	pagination: 
	{
		// pagination data (cursor & page info. struct TBD)
	}
}
```

### Example

```jsx
{
	data: [
		
		  { 
			  // user1 deposited 1000, closed by second deposit
		    "id": 1,
		    "ethereum_address": "0xUserAddress1",
		    "neutron_address": null,
		    "position_start_height": 18000000,
		    "position_end_height": 18000100,
		    "amount_shares": 1000,
		    "isTerminated": false,
	
		  },
		  {
			 // user1 deposited another 1000, no withdraw or transfer
		    "position_ind`id": 2,
		    "ethereum_address": "0xUserAddress1",
		    "neutron_address": null,
		    "position_start_height": 18000101,
		    "position_end_height": null,
		    "amount_shares": 2000,
		    "isTerminated": false,

		    
		  },
		  // user2 deposited 1000 and withdrew
		  {
		    "id": 3,
		    "ethereum_address": "0xUserAddress2",
		    "neutron_address": "neutron1abc...",
		    "position_start_height": 18010000,
		    "position_end_height": 18030000,
		    "amount_shares": 1000,
		    "isTerminated": true,

		  }
	]
}
```