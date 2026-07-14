import { getCollection, type CollectionEntry } from 'astro:content';

export type Doc = CollectionEntry<'docs'>;

// Sidebar order, mirrored from astro.config.mjs. Unknown prefixes sort last.
const SECTION_ORDER = ['getting-started', 'guides', 'reference'];

export const SECTION_LABELS: Record<string, string> = {
  'getting-started': 'Getting Started',
  guides: 'Guides',
  reference: 'Reference',
};

export function section(doc: Doc): string {
  return doc.id.split('/')[0];
}

export async function sortedDocs(): Promise<Doc[]> {
  const docs = await getCollection('docs');
  return docs.sort((a, b) => {
    const sa = SECTION_ORDER.indexOf(section(a));
    const sb = SECTION_ORDER.indexOf(section(b));
    if (sa !== sb) return (sa === -1 ? SECTION_ORDER.length : sa) - (sb === -1 ? SECTION_ORDER.length : sb);
    return a.id.localeCompare(b.id);
  });
}

// Full URL of a page's raw-markdown twin, e.g.
// https://jorgenosberg.github.io/agentcfg/guides/link-vs-copy.md
export function markdownUrl(doc: Doc, site: URL | undefined): string {
  return new URL(`${import.meta.env.BASE_URL}${doc.id}.md`, site).href;
}

// A page rendered as standalone markdown: title, description, body.
export function pageMarkdown(doc: Doc): string {
  const lines = [`# ${doc.data.title}`];
  if (doc.data.description) lines.push('', `> ${doc.data.description}`);
  lines.push('', doc.body?.trim() ?? '');
  return lines.join('\n').trimEnd() + '\n';
}
