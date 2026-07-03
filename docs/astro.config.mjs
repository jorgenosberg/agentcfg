import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// This is a GitHub Pages *project* site (jorgenosberg.github.io/agentcfg),
// so `base` must match the repo name or every internal link/asset breaks
// once deployed. Use `import.meta.env.BASE_URL` in custom components —
// never hard-code a leading "/".
export default defineConfig({
  site: 'https://jorgenosberg.github.io',
  base: '/agentcfg/',
  integrations: [
    starlight({
      title: 'agentcfg',
      description:
        'Sync skills, hooks, and instruction files across local AI coding agent configurations.',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/jorgenosberg/agentcfg' },
      ],
      customCss: ['./src/styles/docs.css'],
      // Muted syntax themes + surfaces matching the landing page's install
      // blocks (--agentcfg-* vars are defined in src/styles/docs.css).
      expressiveCode: {
        themes: ['min-dark', 'min-light'],
        styleOverrides: {
          borderRadius: '8px',
          borderColor: 'var(--agentcfg-border)',
          codeBackground: 'var(--agentcfg-code-bg)',
          frames: {
            frameBoxShadowCssValue: 'none',
            terminalBackground: 'var(--agentcfg-code-bg)',
            terminalTitlebarBackground: 'var(--agentcfg-code-bg)',
            terminalTitlebarBorderBottomColor: 'var(--agentcfg-border)',
            terminalTitlebarDotsForeground: 'var(--sl-color-gray-4)',
            editorBackground: 'var(--agentcfg-code-bg)',
            editorActiveTabBackground: 'var(--agentcfg-code-bg)',
            editorTabBarBackground: 'var(--agentcfg-code-bg)',
          },
        },
      },
      editLink: {
        baseUrl: 'https://github.com/jorgenosberg/agentcfg/edit/main/docs/',
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Quick start', slug: 'getting-started/quick-start' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Source layout', slug: 'guides/source-layout' },
            { label: 'Discovery & import', slug: 'guides/discovery-and-import' },
            { label: 'Link vs copy', slug: 'guides/link-vs-copy' },
            { label: 'Sandbox testing', slug: 'guides/sandbox-testing' },
          ],
        },
        {
          label: 'Reference',
          items: [
            // Auto-generated from the Cobra command tree by `make gen-docs` —
            // new commands/subcommands appear here without touching this file.
            { label: 'CLI', items: [{ autogenerate: { directory: 'reference/cli' } }] },
          ],
        },
      ],
    }),
  ],
});
