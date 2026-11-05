import { Pipe, PipeTransform, inject } from '@angular/core';
import { SideView } from '@bindings/apps/desktop';

import { AppSettings } from '../core/settings.service';
import { fullNames } from './names';

/** "Victor Gauthier / David Michel", prefixed with "#12 " when the bibs are shown (Settings). */
@Pipe({ name: 'withBib', pure: false })
export class WithBibPipe implements PipeTransform {
  private readonly settings = inject(AppSettings);

  transform(side: SideView): string {
    const names = fullNames(side);
    return this.settings.showBibs() ? `#${side.bib} ${names}` : names;
  }
}
