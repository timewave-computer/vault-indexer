import { ApiReference } from "@scalar/nextjs-api-reference";
import { getApiDocs } from "@/app/lib";

const config = await getApiDocs()

export const GET = ApiReference(config)
