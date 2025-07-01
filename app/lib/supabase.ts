import { createClient} from '@supabase/supabase-js'
import { type Database } from '@/app/types'

const supabaseUrl = process.env.API_SUPABASE_URL
if (!supabaseUrl) {
    throw new Error('API_SUPABASE_URL is not set')
}
const supabaseAnonKey = process.env.API_SUPABASE_ANON_KEY
if (!supabaseAnonKey) {
    throw new Error('API_SUPABASE_ANON_KEY is not set')
}

export const supabase = createClient<Database>(supabaseUrl, supabaseAnonKey)


