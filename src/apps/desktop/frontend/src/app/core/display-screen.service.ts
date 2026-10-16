import { Injectable, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';
import { DisplayScreenService as Backend, Monitor } from '@bindings/apps/desktop';

/** Drives the display screen window from the main window. */
@Injectable({ providedIn: 'root' })
export class DisplayScreenService {
  readonly isOpen = signal(false);
  readonly monitors = signal<Monitor[]>([]);

  constructor() {
    Events.On('displayScreen.changed', (event) => this.isOpen.set(event.data));
  }

  async refresh(): Promise<void> {
    const [open, monitors] = await Promise.all([Backend.IsOpen(), Backend.ListMonitors()]);
    this.isOpen.set(open);
    this.monitors.set(monitors ?? []);
  }

  /** Opens the display screen, or moves it. An empty id lets Go pick the monitor. */
  open(monitorId = ''): Promise<void> {
    return Backend.Open(monitorId);
  }

  close(): Promise<void> {
    return Backend.Close();
  }
}
