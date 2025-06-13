

import supabase from "@/app/supabase"
import { isAddress } from 'ethers'
import {  timeRangeSchema } from "@/app/types"
import defineRoute from "@omer-x/next-openapi-route-handler";
import { z } from "zod";

 const getRatesResponseSchema = z.object({
  data: z.array(z.object({
    rate: z.string().describe('Rate of the vault'),
    block_number: z.number().describe('Block number of the rate update'),
    block_timestamp: z.string().describe('UTC timestamp of the block')
  })),
})

const MAX_RESULTS = 5000
export const { GET } = defineRoute({
  method: "GET",
  operationId: "getRates",
  tags: ["/v1/vault"],
  summary: "Vault rates",
  description: `Fetches rates for a vault. By default, it fetches rates for the last 30 days. You can specify a time range to fetch rates for. Maximum output is ${MAX_RESULTS} results.`,
  pathParams: z.object({
    vaultAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/).describe('Ethereum address'),
  }),
  queryParams: timeRangeSchema,
  action: async({pathParams, queryParams}) => {
    try {
      const { vaultAddress } = pathParams

      if (!isAddress(vaultAddress)) {
        throw new Error('Vault address is invalid ethereum address')
      }

      const { from, to, order } = queryParams

      if (new Date(from) > new Date(to)) {
        throw new Error("'from' must be earlier than 'to'")
      }

      const query = supabase.from('rate_updates').select(`
        rate,
        block_number,
        block_timestamp
      `).eq('contract_address', vaultAddress).limit(MAX_RESULTS)

     query.order('block_number', { ascending: order === 'asc' }).gte('block_timestamp', from).lte('block_timestamp', to)
   
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
      description: "List of rates",
      content: getRatesResponseSchema
    },
    400: {
      description: "Bad request",
      content: z.object({
        error: z.string(),
      }),
    },
  },

})

