import generateOpenApiSpec from "@omer-x/next-openapi-json-generator";
import { paginationSchema } from "@/app/types";

export const makeOpenApiSpec = async () => {
  const spec = await generateOpenApiSpec({
    paginationSchema,
  });
  
  const config = {
    pageTitle: 'Valence Indexer API',
    content: {
      ...spec,
      info: {
        title: 'ValenceIndexer API',
        description: "API documentation for the Vault Indexer. The Vault Indexer is a service that indexes vaults created on Ethereum using Valence Protocol.",
        version: '1.0.0'
      },
      components: {
        securitySchemes: {
          ApiKeyAuth: {
            type: 'apiKey',
            in: 'header',
            name: 'x-api-key',
            description: 'API key for authentication'
          }
        }
      },
      security: [
        {
          ApiKeyAuth: []
        }
      ]
    },
  }
  return config
};