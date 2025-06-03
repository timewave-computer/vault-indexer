import { createClient} from '@supabase/supabase-js'
import { type Database } from '@/app/types'

const supabaseUrl = process.env.SUPABASE_URL
if (!supabaseUrl) {
    throw new Error('SUPABASE_URL is not set')
}
const supabaseAnonKey = process.env.SUPABASE_ANON_KEY
if (!supabaseAnonKey) {
    throw new Error('SUPABASE_ANON_KEY is not set')
}

const supabase = createClient<Database>(supabaseUrl, supabaseAnonKey)

export default supabase
