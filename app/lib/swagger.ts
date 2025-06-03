import { createSwaggerSpec } from 'next-swagger-doc';

export const getApiDocs =async () => {
  const spec = createSwaggerSpec({
    apiFolder: 'app/v1', // API folder path
    definition: {
      openapi: '3.0.0',
      info: {
        title: 'Vault Indexer API',
        version: '1.0.0',
        description: 'API documentation for the Vault Indexer service',
      },
      servers: [
        {
          url: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:3000',
          description: 'API Server',
        },
      ],
    },
  });
  return spec;
}; 