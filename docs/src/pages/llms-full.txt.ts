import type { APIRoute } from 'astro';
import { sortedDocs, pageMarkdown } from '../lib/llms';

// The entire documentation concatenated into one plain-text file, for
// agents that want everything in a single fetch.
export const GET: APIRoute = async () => {
  const docs = await sortedDocs();
  const body = docs.map(pageMarkdown).join('\n---\n\n');
  return new Response(body, {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};
