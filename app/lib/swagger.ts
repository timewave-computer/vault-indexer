import { createSwaggerSpec } from "next-swagger-doc";
import path from "path";

const apiFolder = path.join(process.cwd(), "/app/v1/");
console.log('apiFolder',apiFolder);

export const getApiDocs = async () => {
  const spec = createSwaggerSpec({
    apiFolder: "/app/v1/", // Use absolute path with process.cwd()
    definition: {
      openapi: "3.0.0",
      info: {
        title: "Vault Indexer API",
        version: "1.0",
        description: "API documentation for the Vault Indexer service",
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
  });
  return spec;
};