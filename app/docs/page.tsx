import generateOpenApiSpec  from '@omer-x/next-openapi-json-generator'
import { paginationSchema } from '@/app/types'
import { ApiDocs } from '@/app/ui';

export default async function DocsHome() {

    const _spec = await generateOpenApiSpec({
        paginationSchema,
      });
      
      const spec = {
        pageTitle: 'Valence Indexer API',
        content: {
          ..._spec,
          info: {
            title: 'Valence Indexer API',
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

    return <ApiDocs spec={spec} />
}


