import supabase from "@/app/supabase"
import { z } from 'zod'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"
import defineRoute from "@omer-x/next-openapi-route-handler";

const getPositionsQuerySchema = paginationSchema.extend({
  owner_address: z.string().optional(),
})

 const getPositionsResponseSchema = z.object({
  data: z.array(z.object({
    id: z.number(),
    amount_shares: z.number(),
    position_start_height: z.number(),
    position_end_height: z.number(),
    owner_address: z.string(),
    withdraw_receiver_address: z.string(),
    created_at: z.string(),
    is_terminated: z.boolean().nullable(),
  })),
})


export const { GET } = defineRoute({
  method: "GET",
  operationId: "getPositions",
  tags: ["/v1/vault"],
  summary: "Positions",
  description: "Get all positions for a vault. Position amounts and start heights are immutable. When a balance changes, a new position is created at block height B, and the old position is closed at block height B-1. If the balance changes to 0, the position will be marked as terminated.",
  pathParams: z.object({
    vaultAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/),
  }),
  queryParams: getPositionsQuerySchema,
  action: async({pathParams, queryParams}) => {

    try {
      const { vaultAddress } = pathParams

      if (!isAddress(vaultAddress)) {
        throw new Error('Vault address is invalid ethereum address')
      }

      const { owner_address, from, limit, order } = queryParams

      if (owner_address && !isAddress(owner_address)) {
        throw new Error('Invalid owner address')
      }

      const query = supabase.from('positions').select(`
        id:position_index_id,
        amount_shares,
        position_start_height,
        position_end_height,
        owner_address,
        withdraw_receiver_address,
        created_at,
        is_terminated
    `).eq('contract_address', vaultAddress)
      .limit(Number(limit))

    if (owner_address) {
      query.eq('owner_address', owner_address)
    }
    if (order === 'desc') {
      query.order('position_index_id', { ascending: false }).lte('position_index_id', from)
    } else {
      query.order('position_index_id', { ascending: true }).gte('position_index_id', from)
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
      description: "List of positions",
      content: getPositionsResponseSchema,
    },
    400: {
      description: "Bad request",
      content: z.object({
        error: z.string(),
      }),
    },
  },
})

