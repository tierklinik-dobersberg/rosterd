import { CdkRow, CdkRowDef } from '@angular/cdk/table';
import { CommonModule, NgClass } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  TrackByFunction,
  computed,
  effect,
  inject,
  signal,
  untracked
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import { NgIconsModule } from '@ng-icons/core';
import { lucideAlertTriangle, lucideCheckCircle, lucideFilter, lucideListPlus, lucideMoreVertical, lucidePencil, lucideSend, lucideTrash2, lucideXCircle } from '@ng-icons/lucide';
import { BrnAlertDialogModule } from '@spartan-ng/ui-alertdialog-brain';
import { BrnDialogModule } from '@spartan-ng/ui-dialog-brain';
import { BrnMenuTriggerDirective } from '@spartan-ng/ui-menu-brain';
import { BrnRadioGroupModule } from '@spartan-ng/ui-radiogroup-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { BrnSheetModule } from '@spartan-ng/ui-sheet-brain';
import { BrnTableModule } from '@spartan-ng/ui-table-brain';
import { BrnTooltipModule } from '@spartan-ng/ui-tooltip-brain';
import { HlmAlertDialogModule } from '@tierklinik-dobersberg/angular/alertdialog';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import {
  injectOfftimeService
} from '@tierklinik-dobersberg/angular/connect';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { LayoutService } from '@tierklinik-dobersberg/angular/layout';
import { HlmMenuModule } from '@tierklinik-dobersberg/angular/menu';
import {
  DisplayNamePipe,
  UserColorPipe,
  UserContrastColorPipe
} from '@tierklinik-dobersberg/angular/pipes';
import { HlmRadioGroupModule } from '@tierklinik-dobersberg/angular/radiogroup';
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { HlmSeparatorModule } from '@tierklinik-dobersberg/angular/separator';
import { HlmSheetModule } from '@tierklinik-dobersberg/angular/sheet';
import { HlmTableModule } from '@tierklinik-dobersberg/angular/table';
import { coerceDate } from '@tierklinik-dobersberg/angular/utils/date';
import {
  ApprovalRequestType,
  OffTimeEntry,
  OffTimeType,
  Profile,
} from '@tierklinik-dobersberg/apis';
import { toast } from 'ngx-sonner';

import { ConnectError } from '@connectrpc/connect';
import { HlmAlertModule } from '@tierklinik-dobersberg/angular/alert';
import { HlmBadgeModule } from '@tierklinik-dobersberg/angular/badge';
import { HlmDialogModule, HlmDialogService } from '@tierklinik-dobersberg/angular/dialog';
import { HlmSpinnerModule } from '@tierklinik-dobersberg/angular/spinner';
import { HlmTooltipModule } from '@tierklinik-dobersberg/angular/tooltip';
import { UserAvatarPipe, UserLetterPipe } from 'src/app/common/pipes';
import { injectUserProfiles } from '../common/behaviors';
import { TkdContainerSizeClassDirective, injectContainerSize } from '../common/container';
import { TkdEmptyTableComponent } from '../common/empty-table';
import { SortColumn, TkdTableSortColumnComponent } from '../common/table-sort';
import { TkdRoster2Module } from '../roster2/roster2.module';
import { CreateOfftimeComponent } from './create-offtime/create-offtime.component';
import { OffTimeFilter, OffTimeFilterComponent, emptyFilter } from './offtime-filter/offtime-filter.component';

// Filter function for off-time requests. used in sortFunctions below.
type OffTimeSortFunc = (a: OffTimeEntry, b: OffTimeEntry, profiles: Profile[]) => number;

// Column names for the data-table.
enum Columns {
  User = 'user',
  From = 'from',
  To = 'to',
  Description = 'desc',
  Type = 'type',
  Approval = 'approval',
  CreatedAt = 'createdAt',
  Actions = 'actions',
}

// Available sort functions for the data-table
const sortFunctions: { [key in Columns]?: OffTimeSortFunc } = {
  [Columns.User]: (a, b, profiles) => {
    const au = profiles.find(p => p.user!.id === a.creatorId);
    const bu = profiles.find(p => p.user!.id === b.creatorId);

    const aun = (new DisplayNamePipe).transform(au || null);
    const bun = (new DisplayNamePipe).transform(bu || null);

    return bun.localeCompare(aun);
  },

  [Columns.From]: (a, b) => {
    const at = coerceDate(a.from!).getTime();
    const bt = coerceDate(b.from!).getTime();

    return bt - at
  },

  [Columns.To]: (a, b) => {
    const at = coerceDate(a.to!).getTime();
    const bt = coerceDate(b.to!).getTime();

    return bt - at
  },

  [Columns.Type]: (a, b) => {
    return b.type.valueOf() - a.type.valueOf();
  },

  [Columns.Approval]: (a, b) => {
    const an = a.approval ?
      a.approval.approved ? 2 : 1
      : 0;
    const bn = b.approval ?
      b.approval.approved ? 2 : 1
      : 0;

    return bn - an;
  },

  [Columns.Description]: (a, b) => {
    return (b.description || '').localeCompare(a.description || '')
  },

  [Columns.CreatedAt]: (a, b) => {
    const at = coerceDate(a.createdAt!).getTime();
    const bt = coerceDate(b.createdAt!).getTime();

    return bt - at
  },

} as const;

@Component({
  selector: 'app-offtime',
  standalone: true,
  imports: [
    HlmAlertModule,
    CommonModule,
    TkdRoster2Module,

    FormsModule,
    RouterModule,

    NgIconsModule,

    NgClass,

    HlmButtonModule,
    HlmTableModule,
    BrnTableModule,
    BrnSelectModule,
    HlmSelectModule,
    HlmInputModule,
    HlmIconModule,
    HlmSheetModule,
    HlmAvatarModule,
    HlmRadioGroupModule,
    HlmSeparatorModule,
    BrnTableModule,
    HlmTableModule,
    HlmAlertDialogModule,
    BrnAlertDialogModule,
    HlmMenuModule,
    BrnMenuTriggerDirective,
    HlmBadgeModule,
    HlmSpinnerModule,
    HlmTooltipModule,
    BrnTooltipModule,
    BrnDialogModule,
    HlmDialogModule,

    UserLetterPipe,
    BrnSheetModule,
    BrnRadioGroupModule,
    HlmLabelModule,
    UserColorPipe,
    UserContrastColorPipe,
    TkdTableSortColumnComponent,
    TkdContainerSizeClassDirective,
    TkdEmptyTableComponent,
    UserAvatarPipe,

    OffTimeFilterComponent,

    CdkRow,
    CdkRowDef,
  ],
  providers: provideIcons({ lucideListPlus, lucideFilter, lucideMoreVertical, lucideXCircle, lucideCheckCircle, lucideSend, lucidePencil, lucideTrash2, lucideAlertTriangle }),
  templateUrl: './offtime.component.html',
  styles: [
    `
      :host {
        @apply flex flex-col overflow-hidden flex-grow;
      }
    `,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OfftimeComponent {
  private readonly offTimeService = injectOfftimeService();
  private readonly dialog = inject(HlmDialogService);

  protected readonly layout = inject(LayoutService);
  protected readonly types = OffTimeType;
  protected readonly columns = Columns;
  protected readonly container = injectContainerSize();

  protected readonly _filter = signal<OffTimeFilter | null>(null);
  protected readonly _profiles = injectUserProfiles();
  protected readonly _sort = signal<SortColumn<typeof sortFunctions> | null>(null);
  protected readonly _loading = signal<boolean>(false);

  protected _displayedColumns = computed(() => {
    const columns: Columns[] = [];
    const sm = this.container.sm();
    const md = this.container.md();
    const lg = this.container.lg();
    const xl = this.container.xl();

    columns.push(Columns.User)
    columns.push(Columns.From)
    columns.push(Columns.To)

    if (md) {
      columns.push(Columns.Type)
    }

    if (lg) {
      columns.push(Columns.Description)
    }

    if (md || sm) {
      columns.push(Columns.Approval)
    }

    if (xl) {
      columns.push(Columns.CreatedAt)
    }

    columns.push(Columns.Actions)

    return columns;
  })

  private readonly _entries = signal<OffTimeEntry[]>([]);
  protected readonly _totalCount = computed(() => this._entries().length);
  protected readonly _filteredCount = computed(() => this._filteredAndSortedEntries().length);
  protected readonly _filteredAndSortedEntries = computed(() => {
    const filter = this._filter() || emptyFilter;

    const entries = this._entries();
    const sort = this._sort();
    const profiles = this._profiles();

    const filtered = entries
      .filter((value) => {
        if (filter.state !== 'all') {
          if (value.approval) {
            return false;
          }
        }

        if (!filter.profiles || filter.profiles.length === 0) {
          return true;
        }

        return filter.profiles.includes(value.requestorId);
      });

    if (!sort) {
      return filtered;
    }

    const fn = sortFunctions[sort.column];

    if (!fn) {
      return filtered;
    }

    return filtered.sort((a, b) => {
      const result = fn(a, b, profiles);

      if (sort.direction === 'ASC') {
        return result * -1;
      }

      return result;
    })
  })

  approvalComment = '';
  approvalModalEntry: OffTimeEntry | null = null;
  approvalModalApprove: 'approve' | 'reject' = 'approve';

  trackEntry: TrackByFunction<OffTimeEntry> = (_, e) => e.id;

  constructor() {
    effect(() => this.loadOffTimeEntries());
  }

  async loadOffTimeEntries() {
    const filter = this._filter() || emptyFilter;
    const rangeFilter = filter.timeRange;

    untracked(() => {
      if (rangeFilter) {
        if (rangeFilter[0]) {
          rangeFilter[0].setUTCHours(0);
          rangeFilter[0].setUTCMinutes(0);
          rangeFilter[0].setUTCSeconds(0);
        }
        if (rangeFilter[1]) {
          rangeFilter[1].setUTCHours(23);
          rangeFilter[1].setUTCMinutes(59);
          rangeFilter[1].setUTCSeconds(59);
        }
      }

      this._loading.set(true);

      this.offTimeService
        .findOffTimeRequests({
          from:
            rangeFilter && rangeFilter[0]
              ? Timestamp.fromDate(rangeFilter[0])
              : undefined,
          to:
            rangeFilter && rangeFilter[1]
              ? Timestamp.fromDate(rangeFilter[1])
              : undefined,
        })
        .then((response) => {
          this._entries.set(response.results);
        })
        .catch((error) => {
          toast.error(ConnectError.from(error).message)
        })
        .finally(() => this._loading.set(false));
    })
  }

  approveOrRejectConfirmation(approve: boolean, entry: OffTimeEntry) {
    this.approvalModalApprove = approve ? 'approve' : 'reject';
    this.approvalComment = '';
    this.approvalModalEntry = entry;
  }

  async approveOrReject() {
    if (!this.approvalModalEntry) {
      return;
    }

    try {
      await this.offTimeService.approveOrReject({
        id: this.approvalModalEntry.id,
        comment: this.approvalComment,
        type:
          this.approvalModalApprove === 'approve'
            ? ApprovalRequestType.APPROVED
            : ApprovalRequestType.REJECTED,
      })

      toast.success(`Antrag wurde erfolgreich bearbeitet`)
    } catch (err) {
      toast.error(`Fehler: ${ConnectError.from(err).message}`)
    }

    this.approvalModalEntry = null;

    await this.loadOffTimeEntries();
  }

  async deleteEntry(entry: OffTimeEntry) {
    try {
      await this.offTimeService
        .deleteOffTimeRequest({ id: [entry.id!] });

      toast.success(`Antrag wurde erfolgreich gelöscht`)
    } catch (err) {
      toast.error(`Fehler: ${ConnectError.from(err).message}`)
    }

    this.loadOffTimeEntries();
  }

  openOfftimeDialog(entry?: OffTimeEntry) {
    const ref = this.dialog.open(CreateOfftimeComponent, {
      closeOnBackdropClick: false,
      closeOnOutsidePointerEvents: false,
      context: {
        entry,
      }
    })

    ref.closed$
      .subscribe(() => this.loadOffTimeEntries());
  }
}
