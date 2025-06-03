import { getApiDocs } from "@/app/lib/swagger";


export const GET = async () => {
    const spec = await getApiDocs();
    return Response.json(spec);
}
