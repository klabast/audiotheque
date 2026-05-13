import 'virtual:uno.css';
import './main.css';

// Dark mode toggle
function initThemeToggle() {
	const toggle = document.getElementById('theme-toggle');
	if (!toggle) return;

	const iconLight = document.getElementById('theme-icon-light');
	const iconDark = document.getElementById('theme-icon-dark');
	const label = document.getElementById('theme-label');
	let isDark = document.documentElement.classList.contains('dark');

	function updateUI() {
		if (!iconLight || !iconDark || !label) return;
		iconLight.style.display = isDark ? 'none' : 'block';
		iconDark.style.display = isDark ? 'block' : 'none';
		label.textContent = isDark ? 'Dark' : 'Light';
	}

	updateUI();

	toggle.addEventListener('click', () => {
		isDark = !isDark;
		document.documentElement.classList.toggle('dark', isDark);
		updateUI();
	});
}

// Initialize
if (document.readyState === 'loading') {
	document.addEventListener('DOMContentLoaded', initThemeToggle);
} else {
	initThemeToggle();
}
