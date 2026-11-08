const query = '(max-width: 899px)';

/** Whether the screen is narrow enough to show one pane at a time. Matches the CSS. */
class Ui {
  narrow = $state(typeof matchMedia === 'function' ? matchMedia(query).matches : false);

  /** Unix seconds, updated every half minute, so labels such as "Updated 3m ago" stay true. */
  now = $state(Date.now() / 1000);

  constructor() {
    if (typeof matchMedia === 'function') {
      matchMedia(query).addEventListener('change', (e) => (this.narrow = e.matches));
    }
    if (typeof window !== 'undefined') setInterval(() => (this.now = Date.now() / 1000), 30_000);
  }
}

export const ui = new Ui();
