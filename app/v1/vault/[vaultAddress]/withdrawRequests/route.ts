/**
 * @swagger
 * /v1/vault/{vaultAddress}/withdrawRequests:
 *   get:
 *     summary: Get all withdraw requests for a vault
 *     description: Retrieves withdraw requests for a given vault address with pagination
 *     parameters:
 *       - in: path
 *         name: vaultAddress
 *         required: true
 *         schema:
 *           type: string
 *         description: Ethereum address of the vault
 *       - in: query
 *         name: from
 *         schema:
 *           type: integer
 *         description: Withdraw request ID to use as a cutoff point for pagination (inclusive)
 *       - in: query
 *         name: limit
 *         schema:
 *           type: integer
 *         description: Maximum number of records to return
 *       - in: query
 *         name: order
 *         schema:
 *           type: string
 *           enum: [asc, desc]
 *         description: Sort order for the results
 *     responses:
 *       200:
 *         description: List of withdraw requests
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
 *                         description: Unique identifier of the withdraw request
 *                       amount:
 *                         type: string
 *                         description: Amount to be withdrawn
 *                       block_number:
 *                         type: integer
 *                         description: Block number when the request was created
 *                       owner_address:
 *                         type: string
 *                         description: Address of the request owner
 *                       receiver_address:
 *                         type: string
 *                         description: Address that will receive the withdrawn funds
 *                       created_at:
 *                         type: string
 *                         description: Timestamp when the request was created
 *       400:
 *         description: Invalid request parameters or vault address
 */

import supabase from "@/app/supabase"
import { type NextRequest } from 'next/server'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"

const querySchema = paginationSchema

export async function GET(request: NextRequest,
  { params }: { params: Promise<{ vaultAddress: string }> }
) {
  try {
    const { vaultAddress } = await params

    if (!isAddress(vaultAddress)) {
      throw new Error('Invalid vault address')
    }
    const searchParams = request.nextUrl.searchParams
    const { from, limit, order } = querySchema.parse(Object.fromEntries(searchParams.entries()))

    const query = supabase.from('withdraw_requests').select(`
      id:withdraw_id,
      amount,
      block_number,
      owner_address,
      receiver_address,
      created_at
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
}
