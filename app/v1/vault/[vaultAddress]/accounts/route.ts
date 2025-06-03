import supabase from "@/app/supabase"
import { type NextRequest } from 'next/server'
import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"


const querySchema = paginationSchema

export async function GET(request: NextRequest,
  { params: { vaultAddress } }: { params: { vaultAddress: string } }
) {
  try {

    if (!isAddress(vaultAddress)) {
      throw new Error('Invalid ethereum address')
    }
    const searchParams = request.nextUrl.searchParams
    const { from, limit, order } = querySchema.parse(Object.fromEntries(searchParams.entries()))

    return Response.json({ error: "Not implemented" }, { status: 500 })


  } catch (e) {
    const error = e as Error
    return Response.json({ error: error.message }, { status: 400 })
  }
}
