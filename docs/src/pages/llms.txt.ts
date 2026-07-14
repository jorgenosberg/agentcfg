import type { APIRoute } from 'astro';
import { sortedDocs, section, markdownUrl, SECTION_LABELS } from '../lib/llms';

// https://llmstxt.org/ index: links to the raw-markdown twin of every page.
export const GET: APIRoute = async ({ site }) => {
  const docs = await sortedDocs();

  const lines = [
    '# agentcfg',
    '',
    '> Sync skills, hooks, and instruction files across local AI coding agent configurations (Claude Code, Codex CLI, Copilot CLI, Antigravity, opencode).',
    '',
    `Every docs page is also available as raw markdown by appending \`.md\` to its URL. The full documentation in one file: ${new URL(`${import.meta.env.BASE_URL}llms-full.txt`, site).href}`,
  ];

  let current = '';
  for (const doc of docs) {
    const sec = section(doc);
    if (sec !== current) {
      current = sec;
      lines.push('', `## ${SECTION_LABELS[sec] ?? sec}`, '');
    }
    const desc = doc.data.description ? `: ${doc.data.description}` : '';
    lines.push(`- [${doc.data.title}](${markdownUrl(doc, site)})${desc}`);
  }

  return new Response(lines.join('\n') + '\n', {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};
