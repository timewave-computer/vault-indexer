import { makeOpenApiSpec } from "@/app/lib";

export const GET = async () => {
    const spec = await makeOpenApiSpec();
    return Response.json(spec);
}