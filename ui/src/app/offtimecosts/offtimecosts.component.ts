import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, TrackByFunction, computed, inject, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { ConnectError } from '@connectrpc/connect';
import { NgIconsModule } from '@ng-icons/core';
import { lucideAlertTriangle, lucideListPlus, lucideMoreVertical, lucideTrash2 } from '@ng-icons/lucide';
import { BrnAlertDialogModule } from '@spartan-ng/ui-alertdialog-brain';
import { BrnDialogModule } from '@spartan-ng/ui-dialog-brain';
import { BrnMenuTriggerDirective } from '@spartan-ng/ui-menu-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { BrnTableModule } from '@spartan-ng/ui-table-brain';
import { BrnTooltipModule } from '@spartan-ng/ui-tooltip-brain';
import { HlmAlertModule } from '@tierklinik-dobersberg/angular/alert';
import { HlmAlertDialogModule } from '@tierklinik-dobersberg/angular/alertdialog';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmBadgeModule } from '@tierklinik-dobersberg/angular/badge';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { injectOfftimeService } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule, HlmDialogService } from '@tierklinik-dobersberg/angular/dialog';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmMenuModule } from '@tierklinik-dobersberg/angular/menu';
import { DisplayNamePipe, DurationPipe, ToUserPipe } from '@tierklinik-dobersberg/angular/pipes';
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { HlmSpinnerModule } from '@tierklinik-dobersberg/angular/spinner';
import { HlmTableModule } from '@tierklinik-dobersberg/angular/table';
import { HlmTooltipModule } from '@tierklinik-dobersberg/angular/tooltip';
import { OffTimeCosts, Profile, UserOffTimeCosts } from '@tierklinik-dobersberg/apis';
import { toast } from 'ngx-sonner';
import { UserAvatarPipe, UserLetterPipe } from 'src/app/common/pipes';
import { injectUserProfiles, sortProtoDuration, sortProtoTimestamps, sortUserProfile } from '../common/behaviors';
import { TkdContainerSizeClassDirective, injectContainerSize } from '../common/container';
import { TkdEmptyTableComponent } from '../common/empty-table';
import { SortColumn, TkdTableSortColumnComponent } from '../common/table-sort';
import { CostFilter, OffTimeCostFilterComponent, emptyFilter } from './cost-filter/cost-filter.component';
import { CreatecostsComponent } from './createcosts/createcosts.component';

type OffTimeCostSortFunc = (a: CostEntry, b: CostEntry, profiles: Profile[]) => number;

interface CostEntry {
  cost: OffTimeCosts,
  profile: Profile,
}

enum Columns {
  User = 'user',
  Date = 'date',
  Duration = 'duration',
  Type = 'type',
  Reason = 'reason',
  Comment = 'comment',
  CreatedAt = 'createdAt',
  CreatedBy = 'createdBy',
  Actions = 'actions'
}

const sortFunctions: { [key in Columns]?: OffTimeCostSortFunc } = {
  [Columns.User]: (a, b) => {
    return sortUserProfile(a.profile, b.profile)
  },

  [Columns.Date]: (a, b) => {
    return sortProtoTimestamps(a.cost.date, b.cost.date);
  },

  [Columns.Duration]: (a, b) => {
    return sortProtoDuration(a.cost!.costs!, b.cost!.costs!)
  },

  [Columns.Type]: (a, b) => {
    const av = a.cost.isVacation ? 1 : 0;
    const bv = b.cost.isVacation ? 1 : 0;

    return bv - av;
  },

  [Columns.CreatedAt]: (a, b) => {
    return sortProtoTimestamps(a.cost.createdAt, b.cost.createdAt);
  },

  [Columns.CreatedBy]: (a, b, profiles) => {
    const ua = profiles.find(p => p.user!.id === a.profile.user!.id);
    const ub = profiles.find(p => p.user!.id === b.profile.user!.id);

    return sortUserProfile(ua!, ub!);
  }
} as const;

@Component({
  selector: 'app-offtimecosts',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    RouterModule,
    NgIconsModule,
    HlmAvatarModule,

    HlmIconModule,
    HlmButtonModule,
    HlmDialogModule,
    HlmMenuModule,
    BrnMenuTriggerDirective,
    BrnDialogModule,
    HlmTableModule,
    HlmAlertModule,
    HlmAlertDialogModule,
    BrnAlertDialogModule,
    BrnTableModule,
    BrnSelectModule,
    HlmSelectModule,
    HlmInputModule,
    HlmBadgeModule,
    HlmTooltipModule,
    BrnTooltipModule,
    DisplayNamePipe,
    UserLetterPipe,
    DurationPipe,
    ToUserPipe,
    HlmSpinnerModule,
    TkdTableSortColumnComponent,
    TkdContainerSizeClassDirective,
    TkdEmptyTableComponent,
    OffTimeCostFilterComponent,
    TkdEmptyTableComponent,
    UserAvatarPipe,
  ],
  providers: provideIcons({ lucideListPlus, lucideMoreVertical, lucideTrash2, lucideAlertTriangle }),
  templateUrl: './offtimecosts.component.html',
  styles: [
    `
    :host {
      @apply flex flex-col overflow-hidden flex-grow;
    }
    `
  ],
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class OfftimecostsComponent implements OnInit {
  private readonly offTimeService = injectOfftimeService();
  private readonly dialog = inject(HlmDialogService);

  protected readonly container = injectContainerSize();
  protected readonly _displayedColumns = computed(() => {
    const columns: Columns[] = [
      Columns.User,
      Columns.Date,
      Columns.Duration,
    ]

    if (this.container.sm()) {
      columns.push(Columns.Type);
    }

    if (this.container.md()) {
      columns.push(Columns.Reason)
    }

    if (this.container.lg()) {
      columns.push(Columns.Comment)
    }

    if (this.container.width() >= 1200) {
      columns.push(Columns.CreatedAt)
    }

    if (this.container.width() >= 1401/120) {
      columns.push(Columns.CreatedBy)
    }

    columns.push(Columns.Actions);

    return columns;
  })
  protected readonly columns = Columns;
  protected readonly _loading = signal(false);
  protected readonly _profiles = injectUserProfiles();
  protected readonly _costs = signal<UserOffTimeCosts[]>([]);
  protected readonly _entries = computed(() => {
    const result: CostEntry[] = [];
    const profiles = this._profiles();

    this._costs()
      .forEach(userCost => {
        userCost.costs.forEach(cost => {
          result.push({
            cost: cost,
            profile: profiles.find(user => user!.user!.id === userCost.userId)!,
          })
        })
      })

    return result;
  });

  protected readonly _filter = model<CostFilter>(emptyFilter);

  protected readonly _sort = model<SortColumn<typeof sortFunctions> | null>(null)

  protected readonly _filteredEntries = computed(() => {
    const filter = this._filter();
    const entries = this._entries();

    if (!filter) {
      return entries;
    }

    return entries.filter((cost) => {
      if (filter.profiles && filter.profiles.length > 0 && cost.profile) {
        if (!filter.profiles.includes(cost.profile.user!.id)) {
          return false
        }
      }

      if (filter.type) {
        switch (filter.type) {
          case 'all':
            break;

          case 'timeoff':
            if (cost.cost.isVacation) {
              return false
            }
            break;

          case 'vacation':
            if (!cost.cost.isVacation) {
              return false
            }

            break;
        }
      }

      if (filter.timeRange) {
        const dateSeconds = cost.cost.date?.toDate().getTime() || 0;

        if (!!filter.timeRange[0] && filter.timeRange[0].getTime() > dateSeconds) {
          return false;
        }

        if (!!filter.timeRange[1] && filter.timeRange[1].getTime() < dateSeconds) {
          return false;
        }
      }

      if (filter.reason !== 'all') {
        switch (filter.reason) {
          case 'offtime':
            if (!cost.cost.offtimeId) {
              return false;
            }
            break;

          case 'roster':
            if (!cost.cost.rosterId) {
              return false
            }
            break;
        }
      }

      return true;
    })
  })

  protected readonly _filteredAndSorted = computed(() => {
    const filtered = this._filteredEntries();
    const sort = this._sort();
    const profiles = this._profiles();

    if (!sort) {
      return filtered;
    }

    const fn = sortFunctions[sort.column];
    if (!fn) {
      return filtered
    }

    return [...filtered].sort((a, b,) => {
      const result = fn(a, b, profiles);

      if (sort.direction === 'ASC') {
        return result * -1
      }

      return result;
    })
  })
  protected readonly _totalCount = computed(() => this._entries().length);
  protected readonly _filteredCount = computed(() => this._filteredAndSorted().length);

  protected readonly trackEntries: TrackByFunction<CostEntry> = (_, e) => e.cost.id;


  ngOnInit(): void {
    this._loading.set(true);

    this.loadCosts();
  }

  private loadCosts() {
    this._loading.set(true);
    this.offTimeService.getOffTimeCosts({})
      .then(res => {
        this._costs.set(res.results);
      })
      .catch(err => {
        toast.error(ConnectError.from(err).message);
      })
      .finally(() => this._loading.set(false))
  }

  async delete(id: string) {
    try {
      await this.offTimeService.deleteOffTimeCosts({ ids: [id] })

      this.loadCosts()

      toast.success('Eintrag wurde erfolgreich gelöscht.')
    } catch (err) {
      console.log('wanted to delete id', id)
      toast.error(`Fehler: ${ConnectError.from(err).message}`)
    }
  }

  openCreateDialog() {
    this.dialog.open(CreatecostsComponent, {
      closeOnBackdropClick: false,
      closeOnOutsidePointerEvents: false,
    })
      .closed$
      .subscribe(() => this.loadCosts());
  }
}
