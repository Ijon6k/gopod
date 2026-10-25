import { redirect } from '@sveltejs/kit';

// The Servers page duplicated the Overview and carried fabricated health
// badges, so it was removed. This route survives to retire old bookmarks.
//
// A universal +page.ts load is required, not +page.server.ts: this app is a
// static SPA (adapter-static with an index.html fallback, prerender/ssr
// disabled globally), so server loads never run in the browser and the
// backend answers the missing /servers/__data.json with the index.html
// SPA fallback instead. A universal load runs on the client during SPA
// navigation and throws the redirect itself (SvelteKit 2 semantics).
export function load(): never {
	redirect(307, '/');
}
