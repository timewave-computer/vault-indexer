import { makeOpenApiSpec } from "@/app/lib";
import { ApiReferenceReact } from '@scalar/api-reference-react'
import '@scalar/api-reference-react/style.css'


export default async function DocsHome() {

    const openApiSpec = await makeOpenApiSpec()

    return <ApiReferenceReact configuration={
     openApiSpec
    } />
}


