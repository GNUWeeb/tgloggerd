// The browser entry: read the page data Go embedded, then hydrate the
// server's markup, or render from scratch when the render service was
// down and Go sent the shell (data-ssr="0"); web-ssr.md sections 5 and 7.
import { StrictMode } from 'react';
import { createRoot, hydrateRoot } from 'react-dom/client';
import '@fontsource-variable/inter';
import '@fontsource-variable/jetbrains-mono';
import '@fontsource/roboto/400.css';
import '@fontsource/roboto/500.css';
import '@fontsource/roboto/700.css';
import App from './App.tsx';
import { readPageData } from './pagedata.ts';

const root = document.getElementById('root');
const page = readPageData(document);
if (root && page) {
	const app = <StrictMode><App page={page} /></StrictMode>;
	if (root.dataset.ssr === '1') hydrateRoot(root, app);
	else createRoot(root).render(app);
}
