'use client'

import { AnyApiReferenceConfiguration, ApiReferenceReact } from '@scalar/api-reference-react'
import '@scalar/api-reference-react/style.css'


export function ApiDocs(
    {spec}: {spec: AnyApiReferenceConfiguration}
) {
    return <ApiReferenceReact configuration={
       spec
    } />
}