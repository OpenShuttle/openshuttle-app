import { DestroyRef, Signal, inject, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';

/**
 * A view fetched from Go, refetched on every tournament.updated event until
 * the calling component is destroyed. Must be called in an injection context.
 */
export function liveView<T>(fetch: () => Promise<T>, initial: T): Signal<T> {
  const view = signal<T>(initial);
  const load = () => void fetch().then((v) => view.set(v));
  const off = Events.On('tournament.updated', load);
  inject(DestroyRef).onDestroy(off);
  load();
  return view.asReadonly();
}
