import { Signal, computed, inject } from "@angular/core";
import { LayoutService } from "@tierklinik-dobersberg/angular/layout";

export function injectComputedFilterSheetSide(): Signal<'top' | 'bottom' | 'left' | 'right'> {
  const layout = inject(LayoutService);

  return computed(() => {
    if (layout.md()) {
      return 'right'
    }

    return 'bottom'
  })
}
