import supabase from "@/app/supabase"
import { z } from 'zod'
import { type NextRequest } from 'next/server'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"

/**
 * @swagger
 * /v1/vault/{vaultAddress}/positions:
 *   get:
 *     summary: Get vault positions
 *     description: Retrieves positions for a given vault address with optional filtering and pagination
 *     parameters:
 *       - in: path
 *         name: vaultAddress
 *         required: true
 *         schema:
 *           type: string
 *         description: Ethereum address of the vault
 *       - in: query
 *         name: ethereum_address
 *         schema:
 *           type: string
 *         description: Optional filter by ethereum address
 *       - in: query
 *         name: from
 *         schema:
 *           type: integer
 *         description: Starting position index for pagination
 *       - in: query
 *         name: limit
 *         schema:
 *           type: integer
 *         description: Number of records to return
 *       - in: query
 *         name: order
 *         schema:
 *           type: string
 *           enum: [asc, desc]
 *         description: Sort order
 *     responses:
 *       200:
 *         description: List of positions
 *         content:
 *           application/json:
 *             schema:
 *               type: object
 *               properties:
 *                 data:
 *                   type: array
 *                   items:
 *                     type: object
 *                     properties:
 *                       id:
 *                         type: integer
 *                       amount_shares:
 *                         type: string
 *                       position_start_height:
 *                         type: integer
 *                       position_end_height:
 *                         type: integer
 *                       ethereum_address:
 *                         type: string
 *                       created_at:
 *                         type: string
 *       400:
 *         description: Invalid request parameters
 */

const querySchema = paginationSchema.extend({
  ethereum_address: z.string().optional(),
})

export async function GET(request: NextRequest,
  { params }: { params: Promise<{ vaultAddress: string }> }
) {
  try {
    const { vaultAddress } = await params

    if (!isAddress(vaultAddress)) {
      throw new Error('Invalid vault address')
    }
    const searchParams = request.nextUrl.searchParams
    const { ethereum_address, from, limit, order } = querySchema.parse(Object.fromEntries(searchParams.entries()))

    const query = supabase.from('positions').select(`
      id:position_index_id,
      amount_shares,
      position_start_height,
      position_end_height,
      ethereum_address,
      created_at
  `).eq('contract_address', vaultAddress)
    .limit(Number(limit))


    if (ethereum_address) {
      if (!isAddress(ethereum_address)) {  throw new Error('Invalid ethereum address') }
      query.eq('ethereum_address', ethereum_address)
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
}
