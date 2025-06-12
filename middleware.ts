import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

const API_KEY = process.env.API_AUTH_KEY

if (!API_KEY) {
  throw new Error('API_AUTH_KEY is not set')
}



export function middleware(request: NextRequest) {
  if (process.env.NODE_ENV !== 'development') {
    return checkApiKey(request)
  }

  return NextResponse.next()
}



const checkApiKey = (request: NextRequest) => {

  // Secondary guarantee that only apply to /v1 routes
  if (!request.nextUrl.pathname.includes('/v1')) {
    return NextResponse.next()
  }

  const apiKey = request.headers.get('x-api-key')
  if (!apiKey || apiKey !== API_KEY) {
    return new NextResponse(JSON.stringify({ error: 'Invalid or missing API key' }), { status: 401, headers: { 'content-type': 'application/json' } })
  }
  return NextResponse.next()
}


export const config = {
  matcher: '/v1/:path*', // Primary matcher to ensure only /v1 routes are protected
} 


