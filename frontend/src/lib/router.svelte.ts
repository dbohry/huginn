/** A very small history-based router. Screens read `router.path`. */
class Router {
  path = $state(location.pathname);

  constructor() {
    addEventListener('popstate', () => (this.path = location.pathname));
  }

  navigate(to: string, { replace = false } = {}) {
    if (to === this.path) return;
    if (replace) history.replaceState(null, '', to);
    else history.pushState(null, '', to);
    this.path = to;
  }
}

export const router = new Router();

/**
 * Use on an `<a href>` to move around the app without a page load. The link
 * still works as a normal link when opened in a new tab or with a modifier key.
 */
export function link(node: HTMLAnchorElement) {
  const onclick = (e: MouseEvent) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    if (node.target && node.target !== '_self') return;
    e.preventDefault();
    router.navigate(node.getAttribute('href') ?? '/');
  };
  node.addEventListener('click', onclick);
  return { destroy: () => node.removeEventListener('click', onclick) };
}
