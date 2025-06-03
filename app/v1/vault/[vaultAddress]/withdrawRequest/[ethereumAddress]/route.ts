import supabase from "@/app/supabase"
import { z } from 'zod'
import { type NextRequest } from 'next/server'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"


const querySchema = paginationSchema

export async function GET(request: NextRequest,
  { params: { vaultAddress, ethereumAddress } }: { params: { vaultAddress: string, ethereumAddress: string } }
) {
  try {

    if (!isAddress(vaultAddress)) {
      throw new Error('Invalid vault address')
    }
    if (!isAddress(ethereumAddress)) {
      throw new Error('Invalid ethereum address')
    }

    const searchParams = request.nextUrl.searchParams
    const { from, limit, order } = querySchema.parse(Object.fromEntries(searchParams.entries()))

    const query = supabase.from('withdraw_requests').select(`
      id:withdraw_id,
      amount,
      created_at,
      neutron_address
  `).eq('contract_address', vaultAddress)
    .eq('ethereum_address', ethereumAddress)
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
