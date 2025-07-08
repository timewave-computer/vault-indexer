import { SupabaseClient } from "@supabase/supabase-js";
import { Database } from "@/app/types";
import { BlockTagSchema } from "@/app/lib";



export const getCaseInsensitiveQuery = (value: string) => {
    return value.toLowerCase()
}

export const getBlockNumberFilterForTag = async (
    blockTag: BlockTagSchema,
    supabase: SupabaseClient<Database>,
): Promise<number | undefined> => {
     if (blockTag === 'latest') {
        return undefined
    }
    else {
        return (await _fetchLastValidatedBlockNumber(blockTag, supabase))
    }
}

const _fetchLastValidatedBlockNumber = async (
    blockTag: BlockTagSchema,
    supabase: SupabaseClient<Database>
): Promise<number> => {
  
    if (!blockTag) {
        throw new Error('Block tag not specified');
    }
 
    try {
        const {data, error} = await supabase.from('block_finality').select('last_validated_block_number').eq('block_tag', blockTag)
        console.log('data',data)
        if (error) {
            throw new Error('Could not fetch finalized block', { cause: error as Error });
        }
        if (!data) {
            throw new Error('Could not fetch finalized block');
        }
        if (data.length === 0) {
            throw new Error('Not validated block number for tag');
        }
        else return data[0].last_validated_block_number
       
    } catch (error) {
        throw new Error(`Could not fetch last validated ${blockTag} block`, { cause: error as Error });
    }
};
