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
 *         name: owner_address
 *         schema:
 *           type: string
 *         description: Optional filter by owner ethereum address
 *       - in: query
 *         name: from
 *         schema:
 *           type: integer
 *         description: Starting position index for pagination (position_index_id)
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
 *         description: Sort order for position_index_id
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
 *                         description: Position index ID
 *                       amount_shares:
 *                         type: string
 *                         description: Amount of shares in the position
 *                       position_start_height:
 *                         type: integer
 *                         description: Block height when position was created
 *                       position_end_height:
 *                         type: integer
 *                         description: Block height when position was closed (if applicable)
 *                       owner_address:
 *                         type: string
 *                         description: Ethereum address of the position owner
 *                       withdraw_reciever_address:
 *                         type: string
 *                         description: Ethereum address that will receive withdrawn funds
 *                       created_at:
 *                         type: string
 *                         format: date-time
 *                         description: Timestamp when the position was created
 *       400:
 *         description: Invalid request parameters
 *         content:
 *           application/json:
 *             schema:
 *               type: object
 *               properties:
 *                 error:
 *                   type: string
 *                   description: Error message describing what went wrong
 */

const querySchema = paginationSchema.extend({
  owner_address: z.string().optional(),
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
    const { owner_address, from, limit, order } = querySchema.parse(Object.fromEntries(searchParams.entries()))

    const query = supabase.from('positions').select(`
      id:position_index_id,
      amount_shares,
      position_start_height,
      position_end_height,
      owner_address,
      withdraw_reciever_address,
      created_at
  `).eq('contract_address', vaultAddress)
    .limit(Number(limit))


    if (owner_address) {
      if (!isAddress(owner_address)) {  throw new Error('Invalid owner address') }
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
}
