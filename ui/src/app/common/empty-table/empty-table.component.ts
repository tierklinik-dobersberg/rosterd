import { lucideAlertTriangle, lucideInbox } from '@ng-icons/lucide';
import { Component, input } from "@angular/core"
import { HlmAlertModule } from "@tierklinik-dobersberg/angular/alert";
import { HlmButtonModule } from "@tierklinik-dobersberg/angular/button";
import { HlmIconModule, provideIcons } from "@tierklinik-dobersberg/angular/icon";

export interface Filter {
  reset(): void;
  open(): void;
}

@Component({
  // eslint-disable-next-line @angular-eslint/component-selector
  selector: 'tkd-empty-table',
  standalone: true,
  templateUrl: './empty-table.component.html',
  providers: provideIcons({lucideAlertTriangle, lucideInbox}),
  imports: [
    HlmAlertModule,
    HlmButtonModule,
    HlmIconModule,
  ],
})
export class TkdEmptyTableComponent {
  readonly loading = input(false);
  readonly totalCount = input.required<number>()
  readonly filteredCount = input.required<number>();
  readonly filter = input<Filter | null>(null);
}
