'use client'

import '@scalar/api-reference-react/style.css'
import { type AnyApiReferenceConfiguration, ApiReferenceReact } from '@scalar/api-reference-react'


export function ApiDocs(
    {spec}: {spec: AnyApiReferenceConfiguration}
) {
    return <ApiReferenceReact configuration={
        {
            content: spec
        }
    } />
}