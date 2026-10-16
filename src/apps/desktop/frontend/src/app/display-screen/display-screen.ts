import { Component } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { DisplayView, MatchView, SideView, TournamentService } from '@bindings/apps/desktop';

import { liveView } from '../core/live-view';
import { CategoryBadge } from '../shared/category-badge';

/** Display screen: second window on the hall monitor. Read-only, read from 10 m. */
@Component({
  selector: 'app-display-screen',
  imports: [TranslocoDirective, CategoryBadge],
  templateUrl: './display-screen.html',
  styleUrl: './display-screen.scss',
})
export class DisplayScreen {
  protected readonly view = liveView<DisplayView>(() => TournamentService.GetDisplayView(), {
    calls: [],
    courts: [],
  });

  protected names(side: SideView): string {
    return (side.names ?? []).join(' / ');
  }

  protected trackCall(m: MatchView): string {
    return m.id;
  }
}
