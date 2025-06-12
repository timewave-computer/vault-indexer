import generateOpenApiSpec from "@omer-x/next-openapi-json-generator";
import { paginationSchema } from "@/app/types";
import path from "path";

const apiFolder = path.join(process.cwd(), "/app/v1/");
console.log('apiFolder',apiFolder);

export const getApiDocs = async () => {
  const spec = await generateOpenApiSpec({
    paginationSchema,
  });

  console.log('spec',JSON.stringify(spec, null, 2))
  
  const config = {
    content: {...spec,
      info: {
        title: 'Vault Indexer API',
        description: 'API documentation for the Vault Indexer',
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