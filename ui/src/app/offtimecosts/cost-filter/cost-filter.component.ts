import { BrnRadioGroupModule } from '@spartan-ng/ui-radiogroup-brain';
import { BrnSeparatorModule } from '@spartan-ng/ui-separator-brain';
import { ChangeDetectionStrategy, Component, ElementRef, Input, ViewChild, computed, inject, input, model, output } from "@angular/core";
import { lucideFilter } from '@ng-icons/lucide';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { BrnSheetModule } from '@spartan-ng/ui-sheet-brain';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmButtonModule } from "@tierklinik-dobersberg/angular/button";
import { HlmIconModule, provideIcons } from "@tierklinik-dobersberg/angular/icon";
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { DisplayNamePipe, ToUserPipe } from "@tierklinik-dobersberg/angular/pipes";
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { HlmSheetModule } from "@tierklinik-dobersberg/angular/sheet";
import { Profile } from "@tierklinik-dobersberg/apis/idm/v1";
import { injectComputedFilterSheetSide } from "@tierklinik-dobersberg/angular/behaviors";
import { UserAvatarPipe, UserLetterPipe } from "@tierklinik-dobersberg/angular/pipes";
import { HlmSeparatorModule } from '@tierklinik-dobersberg/angular/separator';
import { LayoutService } from '@tierklinik-dobersberg/angular/layout';
import { HlmRadioGroupModule } from '@tierklinik-dobersberg/angular/radiogroup';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { FormsModule } from '@angular/forms';

export type CostType = 'vacation' | 'timeoff' | 'auto' | 'all';
export type CostReason = 'roster' | 'offtime' | 'all';
export interface CostFilter {
  profiles: string[];
  timeRange: [Date, Date] | null;
  type: CostType;
  reason: CostReason;
}

export const emptyFilter: CostFilter = {
  profiles: [],
  timeRange: null,
  type: 'all',
  reason: 'all',
}

@Component({
  selector: 'app-cost-filter',
  standalone: true,
  templateUrl: './cost-filter.component.html',
  imports: [
    HlmSheetModule,
    BrnSheetModule,
    HlmInputModule,
    HlmSelectModule,
    BrnSelectModule,
    HlmButtonModule,
    HlmIconModule,
    HlmAvatarModule,
    DisplayNamePipe,
    FormsModule,
    BrnSeparatorModule,
    HlmRadioGroupModule,
    BrnRadioGroupModule,
    NzDatePickerModule,
    HlmSeparatorModule,
    ToUserPipe,
    UserLetterPipe,
    UserAvatarPipe,
  ],
  providers: provideIcons({lucideFilter}),
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OffTimeCostFilterComponent {
  protected readonly _layout = inject(LayoutService);

  @ViewChild('triggerButton', { read: ElementRef, static: true})
  protected triggerButton!: ElementRef<HTMLButtonElement>;

  /** All user profiles */
  readonly profiles = input.required<Profile[]>();

  @Input()
  set filter(fn: CostFilter) {
    this._profileFilter.set(fn.profiles);
    this._timeRangeFilter.set(fn.timeRange);
    this._typeFilter.set(fn.type);
    this._reasonFilter.set(fn.reason);
  }
  get filter() {
    return this._computedCurrentFilter();
  }

  /** Emits when the filter should be applied */
  readonly filterChange = output<CostFilter>()

  protected readonly _profileFilter = model<string[]>([]);
  protected readonly _timeRangeFilter = model<[Date, Date] | null>(null);
  protected readonly _typeFilter = model<CostType>('all')
  protected readonly _reasonFilter = model<CostReason>('all')

  protected readonly _computedCurrentFilter = computed(() => {
    const filter: CostFilter = {
      profiles: this._profileFilter(),
      timeRange: this._timeRangeFilter(),
      type: this._typeFilter(),
      reason: this._reasonFilter(),
    }

    return filter;
  })

  protected readonly _computedIsEmptyFilter = computed(() => {
    const current = this._computedCurrentFilter();

    return (!current.profiles || current.profiles.length === 0)
      && current.timeRange === null
      && current.type === 'all'
      && current.reason === 'all';
  })

  protected readonly _computedButtonVariant = computed(() => {
    const isEmpty = this._computedIsEmptyFilter();

    if (isEmpty) {
      return 'outline';
    }

    return 'secondary';
  })

  protected readonly _computedSheetSide = injectComputedFilterSheetSide();

  open() {
    this.triggerButton.nativeElement.click();
  }

  reset() {
    this.filter = emptyFilter;
    this.emit();
  }

  protected emit() {
    this.filterChange.emit(this._computedCurrentFilter());
  }
}
