import { mount } from 'svelte';
import './theme.css';
import App from './App.svelte';
import { applyCachedTheme } from './lib/appearance.svelte';

applyCachedTheme();
mount(App, { target: document.getElementById('app')! });
