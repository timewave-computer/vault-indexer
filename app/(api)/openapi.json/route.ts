import { getApiDocs } from "@/app/lib";


export const GET = async () => {
    const spec = await getApiDocs();
    return Response.json(spec);
}
