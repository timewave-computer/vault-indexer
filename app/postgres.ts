import postgres from 'postgres'
const connectionString = process.env.API_POSTGRES_CONNECTION_STRING

if (!connectionString) {
  throw new Error('API_POSTGRES_CONNECTION_STRING is not set')
}
export const sql = postgres(connectionString)
