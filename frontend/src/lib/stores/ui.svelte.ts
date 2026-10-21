// ──────────────────────────────────────────────
// GOPOD UI Store — Theme State
// ──────────────────────────────────────────────

export type Theme = 'dark' | 'light';

class UIStore {
	theme = $state<Theme>('dark');

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
				document.documentElement.setAttribute('data-mode', 'light');
				document.documentElement.classList.add('light');
				document.documentElement.classList.remove('dark');
			} else {
				document.documentElement.setAttribute('data-theme', 'dark');
				document.documentElement.setAttribute('data-mode', 'dark');
				document.documentElement.classList.add('dark');
				document.documentElement.classList.remove('light');
			}
		}
	}
}

export const ui = new UIStore();
export const uiStore = ui;
