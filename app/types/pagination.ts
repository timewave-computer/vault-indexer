import { z } from "zod";

export const paginationSchema = z.object({
    from: z.string().optional().default('0'),
    limit: z.string().optional().default('100'),
    order: z.enum(['asc', 'desc']).optional().default('asc'),
})