import { ChangeDetectionStrategy, Component, computed, inject, input, model } from "@angular/core";
import { lucideArrowDown, lucideArrowUp, lucideArrowUpDown } from "@ng-icons/lucide";
import { hlm } from '@spartan-ng/ui-core';
import { BrnColumnDefComponent } from '@spartan-ng/ui-table-brain';
import { HlmButtonDirective } from "@tierklinik-dobersberg/angular/button";
import { HlmIconModule, provideIcons } from "@tierklinik-dobersberg/angular/icon";
import { HlmThComponent } from "@tierklinik-dobersberg/angular/table";
import { clsx } from 'clsx';

export type SortDirection = 'ASC' | 'DESC';

export interface SortColumn<T extends Record<string, unknown>> {
  column: keyof T;
  direction: SortDirection;
}

const sortClasses = {
  'ASC': {
    icon: "lucideArrowDown",
    class: 'text-primary rotate-180',
  },
  'DESC': {
    icon: "lucideArrowDown",
    class: 'text-primary',
  },
  null: {
    icon: "lucideArrowUpDown",
    class: "text-primary/25"
  }
} as const;

@Component({
  // eslint-disable-next-line @angular-eslint/component-selector
  selector: 'tkd-sort-th',
  standalone: true,
  imports: [
    HlmButtonDirective,
    HlmIconModule,
  ],
  styles: [
    `:host {
      @apply block w-full;
    }`
  ],
  // eslint-disable-next-line @angular-eslint/no-host-metadata-property
  host: {
    '[class]': '_computedHostClass()'
  },
  providers: provideIcons({ lucideArrowUp, lucideArrowDown, lucideArrowUpDown }),
  template: `
    <button hlmBtn size="sm" variant="ghost" (click)="update()" class="w-full flex flex-row flex-nowrap justify-between overflow-hidden">
      <span [class]="_computedContentClass()">
        <ng-content />
      </span>

      <hlm-icon [name]="_computedIcon()" size="sm" [ngIconClass]="_computedClass()" class="ml-3 flex-shrink-0 flex-grow-0"/>
    </button>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class TkdTableSortColumnComponent<T extends Record<string, unknown>> {
  private readonly cellDef = inject(BrnColumnDefComponent);
  private readonly hlmTh = inject(HlmThComponent, { optional: true });

  /** The currently active sort direction */
  current = model.required<SortColumn<T> | null>();

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  userClass = input<any>('', { alias: 'class' });

  protected readonly _computedContentClass = computed(() => {
    return hlm(clsx('text-ellipsis overflow-hidden whitespace-nowrap', this.userClass()));
  })

  protected readonly _computedHostClass = computed(() => {
    if (this.hlmTh?.truncate()) {
      return '-ml-[0.8rem]'
    }

    return '-ml-4'
  })

  protected readonly _computedVariant = computed(() => {
    const current = this.current();
    let variant: keyof (typeof sortClasses);

    if (!current || current.column !== this.cellDef.name) {
      variant = 'null';
    } else {
      variant = current.direction;
    }

    return sortClasses[variant];
  });

  protected readonly _computedIcon = computed(() => this._computedVariant().icon);
  protected readonly _computedClass = computed(() => {
    const result = hlm(this._computedVariant().class, "transform transition duration-150 ease-in-ou")

    return result;
  });

  protected update() {
    let current = this.current();

    if (current && this.cellDef.name === current.column) {
      if (current.direction === 'ASC') {
        current = {
          column: current.column,
          direction: 'DESC'
        }
      } else {
        current = null;
      }
    } else {
      current = {
        column: this.cellDef.name,
        direction: 'ASC',
      }
    }

    this.current.set(current);
  }
}
