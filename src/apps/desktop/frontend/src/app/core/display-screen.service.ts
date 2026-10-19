import { Injectable, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';
import { DisplayScreenService as Backend, Monitor } from '@bindings/apps/desktop';

/** Drives the display screen window from the main window. */
@Injectable({ providedIn: 'root' })
export class DisplayScreenService {
  readonly isOpen = signal(false);
  /** The display screen is open in full screen (false: in a window). */
  readonly fullscreen = signal(false);
  readonly monitors = signal<Monitor[]>([]);

  constructor() {
    Events.On('displayScreen.changed', (event) => this.isOpen.set(event.data));
    Events.On('displayScreen.fullscreen', (event) => this.fullscreen.set(event.data));
  }

  async refresh(): Promise<void> {
    const [open, fullscreen, monitors] = await Promise.all([Backend.IsOpen(), Backend.IsFullscreen(), Backend.ListMonitors()]);
    this.isOpen.set(open);
    this.fullscreen.set(fullscreen);
    this.monitors.set(monitors ?? []);
  }

  /** Opens the display screen, or moves it. An empty id lets Go pick the monitor. */
  open(monitorId = ''): Promise<void> {
    return Backend.Open(monitorId);
  }

  /** Full screen, or back in a window to move it or reach the desktop. */
  setFullscreen(on: boolean): Promise<void> {
    return Backend.SetFullscreen(on);
  }

  close(): Promise<void> {
    return Backend.Close();
  }
}
