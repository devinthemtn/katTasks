import { vitePreprocess } from '@sveltejs/vite-plugin-svelte'

export default {
  // Consult https://svelte.dev/docs#compile-time-svelte-preprocess
  // for more information about preprocessors
  preprocess: vitePreprocess(),
  compilerOptions: {
    // The window sends every key press to the task list (as the terminal
    // app does), so clickable rows need no key handlers of their own.
    warningFilter: (w) => !w.code.startsWith('a11y_click_events') && w.code !== 'a11y_interactive_supports_focus',
  },
}
