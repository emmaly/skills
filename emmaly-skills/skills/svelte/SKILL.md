---
name: svelte
description: This skill should be used when working in frontend code. Covers SvelteKit + Svelte 5 runes, Tailwind CSS, DaisyUI, TypeScript, Go-served static builds, and Vitest testing.
---

## Conventions

- When paired with a Go backend, SvelteKit builds to static/SSR output served by the Go server. Do not run a Node.js host in production

## Testing

- Use Vitest for SvelteKit
- Write tests when they provide real value, not for coverage metrics
- Focus on logic that is complex, error-prone, or critical
