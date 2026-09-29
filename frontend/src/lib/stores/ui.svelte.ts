// ──────────────────────────────────────────────
// GOPOD UI Store — Theme & Sidebar State
// ──────────────────────────────────────────────

export type Theme = 'dark' | 'light';

class UIStore {
	theme = $state<Theme>('dark');
	sidebarCollapsed = $state(false);
	sidebarMobileOpen = $state(false);

	init() {
		if (typeof window !== 'undefined') {
			// Initialize theme from storage or system preference
			const savedTheme = localStorage.getItem('gopod-theme') as Theme | null;
			if (savedTheme) {
				this.theme = savedTheme;
			} else if (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches) {
				this.theme = 'light';
			}
			this.applyTheme();

			// Initialize sidebar collapse state
			const savedSidebar = localStorage.getItem('gopod-sidebar-collapsed');
			if (savedSidebar !== null) {
				this.sidebarCollapsed = savedSidebar === 'true';
			}
		}
	}

	setTheme(theme: Theme) {
		this.theme = theme;
		if (typeof window !== 'undefined') {
			localStorage.setItem('gopod-theme', this.theme);
			this.applyTheme();
		}
	}

	toggleTheme() {
		this.setTheme(this.theme === 'dark' ? 'light' : 'dark');
	}

	private applyTheme() {
		if (typeof document !== 'undefined') {
			if (this.theme === 'light') {
				document.documentElement.setAttribute('data-theme', 'light');
				document.documentElement.classList.add('light');
				document.documentElement.classList.remove('dark');
			} else {
				document.documentElement.setAttribute('data-theme', 'dark');
				document.documentElement.classList.add('dark');
				document.documentElement.classList.remove('light');
			}
		}
	}

	toggleSidebar() {
		if (typeof window !== 'undefined' && window.innerWidth < 768) {
			this.sidebarMobileOpen = !this.sidebarMobileOpen;
		} else {
			this.sidebarCollapsed = !this.sidebarCollapsed;
			if (typeof window !== 'undefined') {
				localStorage.setItem('gopod-sidebar-collapsed', String(this.sidebarCollapsed));
			}
		}
	}

	closeMobileSidebar() {
		this.sidebarMobileOpen = false;
	}
}

export const ui = new UIStore();
