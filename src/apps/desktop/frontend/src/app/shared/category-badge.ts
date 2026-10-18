import { Component, input } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { Category } from '@bindings/internal/engine';

/**
 * Colored category badge (MD/DH…), colored by the category's slot. Projected
 * content follows the code: `DH · Pool A`.
 */
@Component({
  selector: 'app-category-badge',
  imports: [TranslocoDirective],
  template: `<span
    *transloco="let t"
    class="badge"
    [style.background]="'var(--color-category-' + slot() + ')'"
    [attr.title]="t('category.name.' + category())"
    [class.ring]="ring()"
    >{{ t('category.short.' + category()) }}<ng-content /></span
  >`,
  styles: `
    :host {
      display: inline-flex;
      min-width: 0;
      max-width: 100%;
    }

    .badge {
      display: inline-block;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      color: #fff;
      border-radius: 5px;
      padding: 3px 8px;
      font-size: 12px;
      font-weight: 800;
      line-height: 1.2;
      white-space: nowrap;
    }

    .ring {
      box-shadow: 0 0 0 2px #fff;
    }
  `,
})
export class CategoryBadge {
  readonly category = input.required<Category>();
  readonly slot = input.required<number>();
  /** White outline, for a badge on a colored background. */
  readonly ring = input(false);
}
