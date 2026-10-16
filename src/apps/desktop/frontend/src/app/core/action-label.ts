import { Injectable, inject } from '@angular/core';
import { TranslocoService } from '@jsverse/transloco';
import { ActionView } from '@bindings/apps/desktop';

/** Translates history entries: "history.<Type>" with their params. */
@Injectable({ providedIn: 'root' })
export class ActionLabels {
  private readonly transloco = inject(TranslocoService);

  label(action: ActionView): string {
    const params = this.params(action.params ?? {});
    if (action.type !== 'ActionUndone') {
      return this.transloco.translate(`history.${action.type}`, params);
    }
    const undone = this.transloco.translate(`historyShort.${params['action']}`, params);
    return this.transloco.translate('history.ActionUndone', { action: undone });
  }

  /** Category and priority codes are translated before use as params. */
  private params(params: Record<string, string | undefined>): Record<string, string> {
    const out: Record<string, string> = {};
    for (const [key, value] of Object.entries(params)) {
      out[key] = value ?? '';
      if (key === 'category' && value) {
        out[key] = this.transloco.translate(`category.short.${value}`);
      } else if (key === 'priority' && value) {
        out[key] =
          value === 'balanced'
            ? this.transloco.translate('priority.balanced')
            : this.transloco.translate('priority.first', {
                category: this.transloco.translate(`category.short.${value}`),
              });
      }
    }
    return out;
  }
}
