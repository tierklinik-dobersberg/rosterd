import { ChangeDetectionStrategy, Component, ElementRef, Input, ViewChild, computed, inject, input, output, signal } from "@angular/core";
import { FormsModule } from '@angular/forms';
import { lucideFilter } from '@ng-icons/lucide';
import { BrnRadioGroupModule } from '@spartan-ng/ui-radiogroup-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { BrnSeparatorModule } from '@spartan-ng/ui-separator-brain';
import { BrnSheetModule } from '@spartan-ng/ui-sheet-brain';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmButtonModule } from "@tierklinik-dobersberg/angular/button";
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { LayoutService } from "@tierklinik-dobersberg/angular/layout";
import { DisplayNamePipe, ToUserPipe } from '@tierklinik-dobersberg/angular/pipes';
import { HlmRadioGroupModule } from '@tierklinik-dobersberg/angular/radiogroup';
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { HlmSeparatorModule } from "@tierklinik-dobersberg/angular/separator";
import { HlmSheetModule } from "@tierklinik-dobersberg/angular/sheet";
import { Profile } from "@tierklinik-dobersberg/apis";
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { injectComputedFilterSheetSide } from "src/app/common/behaviors";
import { UserAvatarPipe } from "src/app/common/pipes";

export type OffTimeState = 'all' | 'new';

export interface OffTimeFilter {
  profiles: string[];
  timeRange: [Date, Date] | null;
  state: OffTimeState;
}

export const emptyFilter: OffTimeFilter = {
  profiles: [],
  timeRange: null,
  state: 'all',
} as const;

@Component({
  selector: 'app-offtime-filter',
  standalone: true,
  exportAs: "appOffTimeFilter",
  imports: [
    HlmSheetModule,
    HlmButtonModule,
    HlmSeparatorModule,
    BrnSeparatorModule,
    BrnRadioGroupModule,
    HlmRadioGroupModule,
    BrnSelectModule,
    HlmSelectModule,
    FormsModule,
    HlmAvatarModule,
    HlmIconModule,
    BrnSheetModule,
    NzDatePickerModule,
    HlmInputModule,
    UserAvatarPipe,

    DisplayNamePipe,
    ToUserPipe,
  ],
  providers: [
    ...provideIcons({ lucideFilter }),
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './offtime-filter.component.html',
})
export class OffTimeFilterComponent {
  protected readonly _profileFilter = signal<string[]>([]);
  protected readonly _stateFilter = signal<OffTimeState>('all');
  protected readonly _timeRangeFilter = signal<[Date, Date] | null>(null);
  protected readonly _layout = inject(LayoutService);

  @ViewChild('button', { static: true, read: ElementRef })
  protected buttonElement!: ElementRef<HTMLButtonElement>;

  protected readonly _computedSheetSide = injectComputedFilterSheetSide();

  protected readonly _computedIsEmptyFilter = computed(() => {
    const current = this._currentFilter();

    return (!current.profiles || current.profiles.length === 0)
      && current.state === 'all'
      && current.timeRange === null;
  })

  protected readonly _computedButtonVariant = computed(() => {
    const isEmpty = this._computedIsEmptyFilter();

    if (isEmpty) {
      return 'outline'
    }

    return 'secondary'
  })

  protected readonly _currentFilter = computed(() => {
    const profiles = this._profileFilter();
    const state = this._stateFilter();
    const timeRange = this._timeRangeFilter();

    return {
      profiles,
      timeRange,
      state,
    }
  })

  readonly profiles = input.required<Profile[]>();

  @Input()
  set filter(fn: OffTimeFilter) {
    this._profileFilter.set(fn.profiles);
    this._stateFilter.set(fn.state);
    this._timeRangeFilter.set(fn.timeRange);
  }

  public readonly filterChange = output<OffTimeFilter>();

  reset() {
    this.filter = emptyFilter;

    this.filterChange.emit(this._currentFilter());
  }

  open() {
    this.buttonElement.nativeElement.click();
  }

  protected emit() {
    this.filterChange.emit(this._currentFilter());
  }
}
