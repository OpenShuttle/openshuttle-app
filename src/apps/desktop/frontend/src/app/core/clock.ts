import { Injectable, signal } from '@angular/core';

/** Wall clock shown in the top bar, ticking every second. */
@Injectable({ providedIn: 'root' })
export class Clock {
  readonly now = signal(new Date());

  constructor() {
    setInterval(() => this.now.set(new Date()), 1000);
  }
}
