import postgres from 'postgres'
const connectionString = process.env.API_POSTGGRES_CONNECTION_STRING

if (!connectionString) {
  throw new Error('DATABASE_URL is not set')
}
export const sql = postgres(connectionString)
