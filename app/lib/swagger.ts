import { createSwaggerSpec } from "next-swagger-doc";

export const getApiDocs = async () => {
  const spec = createSwaggerSpec({
    apiFolder: "app/(api)/v1/", // Updated to point to the actual API routes directory
    apis: [`${__dirname}/routes/*.ts`],
    definition: {
      openapi: "3.0.0",
      info: {
        title: "Vault Indexer API",
        version: "1.0",
        description: "API documentation for the Vault Indexer service",
      },
      components: {
      },
      security: [],
    },
  });
  return spec;
};