import { Component, input } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { Category } from '@bindings/internal/engine';

/** Colored category badge (MD/DH…), colored by the category's slot. */
@Component({
  selector: 'app-category-badge',
  imports: [TranslocoDirective],
  template: `<span
    *transloco="let t"
    class="badge"
    [style.background]="'var(--color-category-' + slot() + ')'"
    [attr.title]="t('category.name.' + category())"
    >{{ t('category.short.' + category()) }}</span
  >`,
  styles: `
    .badge {
      display: inline-block;
      color: #fff;
      border-radius: 5px;
      padding: 3px 8px;
      font-size: 12px;
      font-weight: 800;
      line-height: 1.2;
      white-space: nowrap;
    }
  `,
})
export class CategoryBadge {
  readonly category = input.required<Category>();
  readonly slot = input.required<number>();
}
