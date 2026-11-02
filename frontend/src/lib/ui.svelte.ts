const query = '(max-width: 899px)';

/** Whether the screen is narrow enough to show one pane at a time. Matches the CSS. */
class Ui {
  narrow = $state(typeof matchMedia === 'function' ? matchMedia(query).matches : false);

  constructor() {
    if (typeof matchMedia === 'function') {
      matchMedia(query).addEventListener('change', (e) => (this.narrow = e.matches));
    }
  }
}

export const ui = new Ui();
