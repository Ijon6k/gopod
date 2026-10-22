import { redirect } from '@sveltejs/kit';

// The Servers page duplicated the Overview and carried fabricated health
// badges, so it was removed. This route survives to retire old bookmarks.
export function load(): never {
	redirect(307, '/');
}
