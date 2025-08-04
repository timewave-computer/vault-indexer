

import { isAddress } from 'ethers'
import defineRoute from "@omer-x/next-openapi-route-handler";
import { z } from "zod";
import {  supabase, paginationSchema, getBlockNumberFilterForTag, getCaseInsensitiveQuery } from "@/app/lib";

const getWithdrawRequestsQuerySchema = paginationSchema.extend({
  owner_address: z.string().regex(/^0x[a-fA-F0-9]{40}$/).optional().describe('Filter by request owner. Ethereum address'),
  receiver_address: z.string().regex(/^neutron1[0-9a-z]{38,59}$/).optional().describe('Filter by receiver address. Neutron address'),
})

const getWithdrawRequestsResponseSchema = z.object({
  data: z.array(z.object({
    id: z.string(),
    amount: z.string(),
    block_number: z.number(),
    owner_address: z.string(),
    receiver_address: z.string(),
  })),
})

export const { GET } = defineRoute({
  method: "GET",
  operationId: "getWithdrawRequests",
  tags: ["/v1/vault"],
  summary: "Withdraw requests",
  description: "Fetches all withdraw requests for a vault. Withdraw requests are created when a user requests to withdraw their shares from the vault.",
  pathParams: z.object({
    vaultAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/).describe('Ethereum address'),
  }),
  queryParams: getWithdrawRequestsQuerySchema,
  action: async({pathParams, queryParams}) => {
    try {
      const { vaultAddress } = pathParams

      if (!isAddress(vaultAddress)) {
        throw new Error('Vault address is invalid ethereum address')
      }
   
      const {from, limit, order ,blockTag, owner_address, receiver_address } = queryParams

      const blockNumberFilter = blockTag ? (await getBlockNumberFilterForTag(blockTag, supabase)) : undefined

      const query = supabase.from('withdraw_requests').select(`
        id:withdraw_id,
        amount,
        block_number,
        owner_address,
        receiver_address
      `).eq('contract_address', getCaseInsensitiveQuery(vaultAddress)).limit(Number(limit))
    
      if (order === 'desc') {
        query.order('withdraw_id', { ascending: false }).lte('withdraw_id', from)
      } else {
        query.order('withdraw_id', { ascending: true }).gte('withdraw_id', from)
      }

      if (blockNumberFilter) {
          query.lte('block_number', blockNumberFilter)
      }

      if (owner_address) {
        query.eq('owner_address', getCaseInsensitiveQuery(owner_address))
      }
      if (receiver_address) {
        query.eq('receiver_address', getCaseInsensitiveQuery(receiver_address))
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
      content: getWithdrawRequestsResponseSchema
    },
    400: {
      description: "Bad request",
      content: z.object({
        error: z.string(),
      }),
    },
  },

})



