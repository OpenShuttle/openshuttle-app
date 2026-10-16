import { Injectable, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';
import { Settings, SettingsService } from '@bindings/apps/desktop';

import { BackendError, backendError } from './backend-error';

/** App settings from app.db, kept in sync across windows. */
@Injectable({ providedIn: 'root' })
export class AppSettings {
  readonly value = signal<Settings | null>(null);
  readonly error = signal<BackendError | null>(null);

  constructor() {
    Events.On('settings.changed', (event) => this.value.set(event.data));
    void SettingsService.GetSettings().then((s) => this.value.set(s));
  }

  /** Runs a setter; Go broadcasts the new settings on success. */
  async run(call: () => Promise<unknown>): Promise<boolean> {
    try {
      await call();
      this.error.set(null);
      return true;
    } catch (err) {
      this.error.set(backendError(err));
      return false;
    }
  }
}
