import { z } from "zod";

export const paginationSchema = z.object({
    from: z.coerce.number().int().min(0).optional().default(0),
    limit: z.coerce.number().int().min(1).optional().default(100),
    order: z.enum(['asc', 'desc']).optional().default('asc'),
})

