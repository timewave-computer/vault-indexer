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
      reciever_address,
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
