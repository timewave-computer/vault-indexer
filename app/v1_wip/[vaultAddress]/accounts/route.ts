import { type NextRequest } from 'next/server'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"

/**
 * @swagger
 * /v1/vault/{vaultAddress}/accounts:
 *   get:
*     summary: Get accounts for a specific vault
 *     description: Retrieves accounts for a given vault address with pagination
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
 *         description: Starting account index for pagination
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
 *         description: List of accounts
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
 *                       address:
 *                         type: string
 *                       created_at:
 *                         type: string
 *       400:
 *         description: Invalid request parameters
 *       500:
 *         description: Not implemented
 */

const querySchema = paginationSchema


export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ vaultAddress: string }> }
) {
  try {
    const { vaultAddress } = await params
    if (!isAddress(vaultAddress)) {
      throw new Error('Invalid ethereum address')
    }
    const searchParams = request.nextUrl.searchParams

    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { from, limit, order } = querySchema.parse(Object.fromEntries(searchParams.entries()))

    return Response.json({ error: "API still in development" }, { status: 500 })


  } catch (e) {
    const error = e as Error
    return Response.json({ error: error.message }, { status: 400 })
  }
}
