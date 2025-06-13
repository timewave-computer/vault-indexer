import { z } from "zod";

const orderSchema = z.enum(['asc', 'desc']).optional().default('asc').describe('Return results in ascending or descending order')

export const paginationSchema = z.object({
    from: z.coerce.number().int().min(0).optional().default(0).describe('Index (ID) from which to start returning results'),
    limit: z.coerce.number().int().min(1).optional().default(100).describe('Number of results to return'),
    order: orderSchema
})

const timeStampSchema = z.coerce.string().datetime().optional()
const thirtyDaysAgo = new Date(Date.now() - (30 * 24 * 60 * 60 * 1000));
export const timeRangeSchema = z.object({
    from: timeStampSchema.default(thirtyDaysAgo.toISOString()).describe('Start of the time range'),
    to: timeStampSchema.default(new Date().toISOString()).describe('End of the time range'),
    order: orderSchema
})
