import supabase from "@/app/supabase"
import { z } from 'zod'
import { type NextRequest } from 'next/server'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"


const querySchema = paginationSchema.extend({
  ethereum_address: z.string().optional(),
})

export async function GET(request: NextRequest,
  { params }: { params: Promise<{ vaultAddress: string }> }
) {
  try {
    const { vaultAddress } = await params

    if (!isAddress(vaultAddress)) {
      throw new Error('Invalid ethereum address')
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

    .order('position_index_id', { ascending: order === 'asc' })

    if (ethereum_address) {
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
