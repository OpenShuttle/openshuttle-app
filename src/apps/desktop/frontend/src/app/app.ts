import { Component, effect, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';

import { AppSettings } from './core/settings.service';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet],
  template: '<router-outlet />',
})
export class App {
  private readonly settings = inject(AppSettings);

  constructor() {
    // styles.scss hides the bib badges unless this class is on the root.
    effect(() => document.documentElement.classList.toggle('show-bibs', this.settings.showBibs()));
  }
}
