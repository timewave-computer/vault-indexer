import { z } from "zod";

export const paginationSchema = z.object({
    from: z.coerce.number().optional().default(0),
    limit: z.coerce.number().optional().default(100),
    order: z.enum(['asc', 'desc']).optional().default('asc'),
})