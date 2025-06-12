

import supabase from "@/app/supabase"
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"
import defineRoute from "@omer-x/next-openapi-route-handler";
import { z } from "zod";

export const getWithdrawRequestsResponseSchema = z.object({
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
  summary: "All withdraw requests",
  description: "Fetches all withdraw requests for a vault. Withdraw requests are created when a user requests to withdraw their shares from the vault.",
  pathParams: z.object({
    vaultAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/),
  }),
  queryParams: paginationSchema,
  action: async({pathParams, queryParams}) => {
    try {
      const { vaultAddress } = pathParams

      if (!isAddress(vaultAddress)) {
        throw new Error('Vault address is invalid ethereum address')
      }

      const { from, limit, order } = queryParams

      const query = supabase.from('withdraw_requests').select(`
        id:withdraw_id,
        amount,
        block_number,
        owner_address,
        receiver_address,
      `).eq('contract_address', vaultAddress)
        .limit(Number(limit))

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

