import type { Page } from './types.js';

/**
 * Creates a page navigation controller with history stack.
 * Shared across desktop, extension, and dev preview entry points.
 */
export function createNavigation(initial: Page = 'main') {
  let page = $state<Page>(initial);
  let history = $state<Page[]>([]);

  return {
    get page() { return page; },
    set page(v: Page) { page = v; },

    navigateTo(target: Page) {
      history.push(page);
      page = target;
    },

    navigateBack() {
      page = history.pop() ?? 'main';
    },

    reset(to: Page = 'main') {
      history = [];
      page = to;
    },
  };
}
