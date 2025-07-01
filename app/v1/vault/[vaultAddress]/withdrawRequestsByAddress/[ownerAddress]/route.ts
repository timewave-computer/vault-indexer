import { isAddress } from 'ethers'
import defineRoute from "@omer-x/next-openapi-route-handler";
import { z } from "zod";
import { getMostRecentBlockNumber, supabase, paginationSchema } from "@/app/lib";

const getWithdrawRequestsByAddressResponseSchema = z.object({
  data: z.array(z.object({
    id: z.string(),
    amount: z.string(),
    owner_address: z.string(),
    receiver_address: z.string(),
    block_number: z.number(),
  })),
})

export const { GET } = defineRoute({
  method: "GET",
  operationId: "getWithdrawRequestsByAddress",
  tags: ["/v1/vault"],
  summary: "Withdraw requests by address",
  description: "Fetches all withdraw requests for a vault by a specific address.",
  pathParams: z.object({
    vaultAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/).describe('Ethereum address'),
    ownerAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/).describe('Ethereum address'),
  }),
  queryParams: paginationSchema,
  action: async({pathParams, queryParams}) => {
    try {
      const { vaultAddress, ownerAddress } = pathParams

      if (!isAddress(vaultAddress)) {
        throw new Error('Vault address is invalid ethereum address')
      }
      if (!isAddress(ownerAddress)) {
        throw new Error('Owner address is invalid ethereum address')
      }

      const { from, limit, order, blockTag } = queryParams

      const blockNumber = blockTag ? (await getMostRecentBlockNumber(blockTag)) : 0


      const query = supabase.from('withdraw_requests').select(`
        id:withdraw_id,
        amount,
        block_number,
        owner_address,
        receiver_address
      `).eq('contract_address', vaultAddress)
        .eq('owner_address', ownerAddress)
        .limit(Number(limit))

      if (queryParams.blockTag) {
        query.lte('block_number', blockNumber)
      }

      if (order === 'desc') {
        query.order('withdraw_id', { ascending: false }).lte('withdraw_id', from)
      } else {
        query.order('withdraw_id', { ascending: true }).gte('withdraw_id', from)
      }

      const { data, error } = await query

      if (error) {
        throw Error(error.message)
      }

      return Response.json({ data })
    } catch (e) {
      const error = e as Error
      return Response.json({ error: error.message }, { status: 400 })
    }
  },
  responses: {
    200: {
      description: "List of withdraw requests",
      content: getWithdrawRequestsByAddressResponseSchema
    },
    400: {
      description: "Bad request",
      content: z.object({
        error: z.string(),
      }),
    },
  },
})