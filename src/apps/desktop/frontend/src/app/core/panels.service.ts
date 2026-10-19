import { Injectable, signal } from '@angular/core';

const STORAGE_KEY = 'openshuttle.panels';

function load(): Record<string, boolean> {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}') as Record<string, boolean>;
  } catch {
    return {};
  }
}

/**
 * Which side panels are collapsed (the tournament menu, the right panels of
 * some screens). A convenience of this PC: kept in local storage, defaults to
 * open when unavailable.
 */
@Injectable({ providedIn: 'root' })
export class Panels {
  private readonly state = signal<Record<string, boolean>>(load());

  collapsed(key: string): boolean {
    return this.state()[key] ?? false;
  }

  toggle(key: string): void {
    this.state.update((s) => ({ ...s, [key]: !(s[key] ?? false) }));
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.state()));
    } catch {
      // Not saved: the panel still toggles for this session.
    }
  }
}
