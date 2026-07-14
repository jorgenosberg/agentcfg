import type { APIRoute } from 'astro';
import { sortedDocs, pageMarkdown, type Doc } from '../lib/llms';

// Every docs page is also served as raw markdown at its own URL with a
// `.md` suffix (…/guides/link-vs-copy/ → …/guides/link-vs-copy.md), so
// agents can read pure content without scraping HTML.
export async function getStaticPaths() {
  const docs = await sortedDocs();
  return docs.map((doc) => ({ params: { slug: doc.id }, props: { doc } }));
}

export const GET: APIRoute<{ doc: Doc }> = ({ props }) => {
  return new Response(pageMarkdown(props.doc), {
    headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
  });
};
