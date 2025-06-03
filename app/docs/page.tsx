import { getApiDocs } from '@/app/lib';
import { ReactSwagger } from '@/app/components';

export default async function ApiDoc() {
    const spec = await getApiDocs();
    return (
      <section className="container">
        <ReactSwagger spec={spec} />
      </section>
    );

} 