import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App.jsx';
import '@fontsource-variable/inter';
import '@fontsource-variable/jetbrains-mono';
import '@fontsource/roboto/400.css';
import '@fontsource/roboto/500.css';
import '@fontsource/roboto/700.css';
import './base.css';
import './a/console.css';
import './b/telegram.css';
import './c/data.css';

createRoot(document.getElementById('root')).render(
	<StrictMode>
		<App />
	</StrictMode>,
);
