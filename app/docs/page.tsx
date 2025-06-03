import { ReactSwagger } from '@/app/components';
import { getApiDocs } from '@/app/lib/swagger';


export default async function ApiDoc() {
  const spec = await getApiDocs();
    return (
      <section className="container">
        <ReactSwagger spec={spec} />
      </section>
    );

} 