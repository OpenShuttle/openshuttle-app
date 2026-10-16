import { DestroyRef, Signal, inject, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';

/**
 * A view fetched from Go, refetched on every tournament.updated event until
 * the calling component is destroyed. Must be called in an injection context.
 *
 * refreshMs also refetches periodically, for views that depend on the time
 * (rest periods end without any event).
 */
export function liveView<T>(fetch: () => Promise<T>, initial: T, refreshMs = 0): Signal<T> {
  const view = signal<T>(initial);
  const load = () => void fetch().then((v) => view.set(v));
  const off = Events.On('tournament.updated', load);
  const timer = refreshMs > 0 ? setInterval(load, refreshMs) : undefined;
  inject(DestroyRef).onDestroy(() => {
    off();
    clearInterval(timer);
  });
  load();
  return view.asReadonly();
}
