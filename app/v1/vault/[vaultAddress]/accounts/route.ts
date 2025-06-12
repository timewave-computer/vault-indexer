import { isAddress } from 'ethers'
import { paginationSchema } from "@/app/types"
import { sql } from '@/app/postgres'
import defineRoute from "@omer-x/next-openapi-route-handler";
import z from "zod";

export const { GET } = defineRoute({
  method: "GET",
  operationId: "accounts",
  tags: ["vault"],
  summary: "Get all ethereum addresses that have held a position in a vault",
  description: "Retrieves accounts for a given vault address with pagination",
  pathParams: z.object({
    vaultAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/),
  }),
  queryParams: paginationSchema,
  action: async({pathParams, queryParams}) => {
    try {
      const vaultAddress  = pathParams.vaultAddress
      if (!isAddress(vaultAddress)) {
        throw new Error('Invalid ethereum address')
      }
  

      const { from, limit, order } = queryParams
  
      const response = await sql`
      SELECT DISTINCT owner_address
      FROM positions
      WHERE contract_address = ${vaultAddress}
      ORDER BY owner_address ${order === 'asc' ? sql`ASC` : sql`DESC`}
      LIMIT ${limit}
      OFFSET ${from}
    `
    return Response.json( response.map(r => r.owner_address), { status: 200 })

    }
    catch (e) {
      const error = e as Error
      return Response.json({ error: error.message }, { status: 400 })
    }


  },
  responses: {
    200: {
      description: "List of account addresses",
      content: z.array(z.string())
      },
    }

})


