import { Injectable, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';
import { AppService, UpdateInfo } from '@bindings/apps/desktop';

import { BackendError, backendError } from './backend-error';

/**
 * Whether a newer release is published. Go checks at startup (when the
 * setting allows it) and on demand; the app never downloads it.
 */
@Injectable({ providedIn: 'root' })
export class AppUpdates {
  readonly info = signal<UpdateInfo>({ checked: false, latest: '', available: false });
  readonly checking = signal(false);
  readonly error = signal<BackendError | null>(null);

  constructor() {
    Events.On('update.checked', (event) => this.set(event.data));
    void AppService.GetUpdate().then((info) => this.set(info));
  }

  private set(info: UpdateInfo | null | undefined): void {
    if (info) {
      this.info.set(info);
    }
  }

  /** Checks now, whatever the setting. */
  async check(): Promise<void> {
    this.checking.set(true);
    try {
      this.set(await AppService.CheckForUpdate());
      this.error.set(null);
    } catch (err) {
      this.error.set(backendError(err));
    } finally {
      this.checking.set(false);
    }
  }
}
