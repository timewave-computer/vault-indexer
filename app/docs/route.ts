import { ApiReference } from "@scalar/nextjs-api-reference";
import { makeOpenApiSpec } from "@/app/lib";

const openApiSpec = await makeOpenApiSpec()

export const GET = ApiReference(openApiSpec)
