import supabase from "@/app/supabase"
import { type NextRequest } from 'next/server'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"

/**
 * @swagger
 * /v1/vault/{vaultAddress}/withdrawRequestByAddress/{ownerAddress}:
 *   get:
 *     summary: Get vault withdraw request by ethereum address
 *     description: Retrieves withdraw requests for a given ethereum address in a specific vault
 *     parameters:
 *       - in: path
 *         name: vaultAddress
 *         required: true
 *         schema:
 *           type: string
 *         description: Ethereum address of the vault
 *       - in: path
 *         name: ownerAddress
 *         required: true
 *         schema:
 *           type: string
 *         description: Owner address to get withdraw requests for
 *       - in: query
 *         name: from
 *         schema:
 *           type: integer
 *         description: Starting withdraw request ID for pagination
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
 *                         description: Withdraw request ID
 *                       amount:
 *                         type: string
 *                         description: Amount to withdraw
 *                       created_at:
 *                         type: string
 *                         description: Timestamp of request creation
 *                       owner_address:
 *                         type: string
 *                         description: Ethereum address of the request owner
 *                       receiver_address:
 *                         type: string
 *                         description: Ethereum address of the receiver
 *                       block_number:
 *                         type: integer
 *                         description: Block number when the request was created
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

const querySchema = paginationSchema

export async function GET(request: NextRequest,
  { params }: { params: Promise<{ vaultAddress: string, ownerAddress: string }> }
) {

  const { vaultAddress, ownerAddress } = await params
  try {

    if (!isAddress(vaultAddress)) {
      throw new Error('Invalid vault address')
    }
    if (!isAddress(ownerAddress)) {
      throw new Error('Invalid owner address')
    }

    const searchParams = request.nextUrl.searchParams
    const { from, limit, order } = querySchema.parse(Object.fromEntries(searchParams.entries()))

    const query = supabase.from('withdraw_requests').select(`
      id:withdraw_id,
      amount,
      created_at,
      owner_address,
      receiver_address,
      block_number
  `).eq('contract_address', vaultAddress)
    .eq('owner_address', ownerAddress)
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
