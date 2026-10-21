/** Where an element was drawn, within its offset parent's parent. */
interface Point {
  x: number;
  y: number;
}

/**
 * Glides the elements of a container to their new place after a render
 * (FLIP): each element marked `data-flip="<key>"` that moved since the last
 * call animates from where it was drawn to where it is, from one column to
 * another included, starting from where it is seen when a glide was running.
 * Call `run()` after every render (afterEveryRender).
 */
export class Glide {
  private positions = new Map<string, Point>();

  constructor(
    private readonly host: HTMLElement,
    private readonly duration = 280,
  ) {}

  run(): void {
    const next = new Map<string, Point>();
    for (const el of this.host.querySelectorAll<HTMLElement>('[data-flip]')) {
      const parent = el.offsetParent as HTMLElement | null;
      const at = { x: el.offsetLeft + (parent?.offsetLeft ?? 0), y: el.offsetTop + (parent?.offsetTop ?? 0) };
      const key = el.dataset['flip'] ?? '';
      next.set(key, at);
      const was = this.positions.get(key);
      if (was && (was.x !== at.x || was.y !== at.y)) {
        this.animate(el, was.x - at.x, was.y - at.y);
      }
    }
    this.positions = next;
  }

  private animate(el: HTMLElement, dx: number, dy: number): void {
    const running = el.getAnimations();
    if (running.length > 0) {
      const m = new DOMMatrix(getComputedStyle(el).transform);
      dx += m.e;
      dy += m.f;
      running.forEach((a) => a.cancel());
    }
    el.animate([{ transform: `translate(${dx}px, ${dy}px)` }, { transform: 'translate(0, 0)' }], {
      duration: this.duration,
      easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)',
    });
  }
}
