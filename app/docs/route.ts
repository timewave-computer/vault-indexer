import { ApiReference } from "@scalar/nextjs-api-reference";

const config = {
  pageTitle: 'Vault Indexer API',
  description: 'API documentation for the Vault Indexer',
    url: '/openapi.json',
  }
export const GET = ApiReference(config)
