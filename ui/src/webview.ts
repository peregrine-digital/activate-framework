/**
 * Webview entry point for the VS Code extension.
 *
 * This is built by Vite into a single JS+CSS bundle that the extension
 * loads into the webview panel. It uses the VS Code adapter for communication.
 */
import { mount } from 'svelte';
import App from './WebviewApp.svelte';
import './app.css';

const app = mount(App, { target: document.getElementById('app')! });

export default app;
