import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, TrackByFunction, computed, inject, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Timestamp } from '@bufbuild/protobuf';
import { ConnectError } from '@connectrpc/connect';
import { lucideMoreVertical, lucidePencil, lucideTrash2 } from '@ng-icons/lucide';
import { BrnAlertDialogModule } from '@spartan-ng/ui-alertdialog-brain';
import { BrnMenuTriggerDirective } from '@spartan-ng/ui-menu-brain';
import { BrnTableModule } from '@spartan-ng/ui-table-brain';
import { HlmAlertDialogModule } from '@tierklinik-dobersberg/angular/alertdialog';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmBadgeModule } from '@tierklinik-dobersberg/angular/badge';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { injectWorktimeSerivce } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule, HlmDialogService } from '@tierklinik-dobersberg/angular/dialog';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmMenuModule } from '@tierklinik-dobersberg/angular/menu';
import { DisplayNamePipe, ToDatePipe } from '@tierklinik-dobersberg/angular/pipes';
import { HlmSpinnerModule } from '@tierklinik-dobersberg/angular/spinner';
import { HlmTableModule } from '@tierklinik-dobersberg/angular/table';
import { AnalyzeVacation, UserVacationSum, UserWorkTime, WorkTime } from '@tierklinik-dobersberg/apis/roster/v1';
import { Profile } from '@tierklinik-dobersberg/apis/idm/v1';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzCheckboxModule } from 'ng-zorro-antd/checkbox';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { NzModalModule } from 'ng-zorro-antd/modal';
import { NzTimelineModule } from 'ng-zorro-antd/timeline';
import { NzToolTipModule } from 'ng-zorro-antd/tooltip';
import { toast } from 'ngx-sonner';
import { filter } from 'rxjs';
import { injectUserProfiles, sortProtoDuration, sortUserProfile } from '../common/behaviors';
import { injectContainerSize } from '../common/container';
import { UserAvatarPipe, UserLetterPipe } from '../common/pipes';
import { SortColumn, TkdTableSortColumnComponent } from '../common/table-sort';
import { TkdRoster2Module } from '../roster2/roster2.module';
import { SetWorktimeDialogComponent } from './set-worktime-dialog';

enum Columns {
  User = 'user',
  Since = 'since',
  Current = 'current',
  TimeTracking = 'timeTracking',
  Vacation = 'vacation',
  EndsWith = 'endsWith',
  VacationLeft = 'vacationLeft',
  Overtime = 'overtime',
  Actions = 'actions',
}

interface Model {
  profile: Profile;
  current?: WorkTime;
  next?: WorkTime;
  credits?: UserVacationSum;
  currentIsNext: boolean;
  currentEnded: boolean;
}

type SortFunc = (a: Model, b: Model) => number;

const sortFns: { [key in Columns]?: SortFunc } = {
  [Columns.User]: (a, b) => {
    return sortUserProfile(a.profile, b.profile)
  },

  [Columns.Since]: (a, b) => {
    const ad = new Date(a.current?.applicableAfter || 0);
    const bd = new Date(b.current?.applicableAfter || 0);

    return bd.getTime() - ad.getTime();
  },

  [Columns.Current]: (a, b) => {
    return sortProtoDuration(a.current?.timePerWeek, b.current?.timePerWeek)
  },

  [Columns.VacationLeft]: (a, b) => {
    return sortProtoDuration(a.credits?.vacationCreditsLeft, b.credits?.vacationCreditsLeft)
  },

  [Columns.Overtime]: (a, b) => {
    return sortProtoDuration(a.credits?.timeOffCredits, b.credits?.timeOffCredits)
  }
}

@Component({
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    TkdRoster2Module,
    NzAvatarModule,
    NzModalModule,
    NzMessageModule,
    NzCheckboxModule,
    NzTimelineModule,
    NzToolTipModule,
    HlmIconModule,
    HlmDialogModule,
    HlmAvatarModule,
    HlmIconModule,
    BrnTableModule,
    HlmTableModule,
    HlmMenuModule,
    BrnMenuTriggerDirective,
    HlmAlertDialogModule,
    BrnAlertDialogModule,
    TkdTableSortColumnComponent,
    HlmButtonModule,
    HlmSpinnerModule,
    UserLetterPipe,
    DisplayNamePipe,
    ToDatePipe,
    HlmBadgeModule,
    UserAvatarPipe,
  ],
  templateUrl: './worktimes.component.html',
  providers: [
    ...provideIcons({
      lucideTrash2,
      lucidePencil,
      lucideMoreVertical,
    }),
  ],
  styles: [
    `
    :host {
      @apply flex flex-col overflow-hidden flex-grow;
    }
    `
  ],
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class WorktimesComponent implements OnInit {
  private readonly workTimeService = injectWorktimeSerivce();
  private readonly dialog = inject(HlmDialogService);

  protected readonly endOfYear = new Date((new Date()).getFullYear(), 11, 31, 23, 59, 59)
  protected readonly columns = Columns;
  protected readonly container = injectContainerSize();

  analyze: AnalyzeVacation | null = null;

  protected readonly _loading = signal<boolean>(false);
  protected readonly _profiles = injectUserProfiles();
  protected readonly _workTimes = signal<UserWorkTime[]>([]);
  protected readonly _credits = signal<UserVacationSum[]>([]);
  protected readonly _sort = model<SortColumn<typeof sortFns> | null>(null)

  protected readonly _computedModels = computed<Model[]>(() => {
    const profiles = this._profiles();
    const workTimes = this._workTimes();
    const credits = this._credits();
    const sort = this._sort();

    const models = profiles
      .map(profile => {
        const wt = workTimes.find(wt => wt.userId === profile.user!.id);

        // FIXME(ppacher): there are some bugs below!

        // FIXME(ppacher): this actually needs to be sorted first
        const realNext = wt?.history.find(h => new Date(h.applicableAfter!).getTime() > new Date(wt.current?.applicableAfter || 0).getTime());

        let current = wt?.current;
        let ended = false;
        // FIXME(ppacher): make sure to check against mighnight in UTC here!
        if (current && current.endsWith && new Date(current.endsWith).getTime() < (new Date()).getTime()) {
          current = undefined;
          ended = true;
        }

        return {
          profile: profile,
          current: current || realNext,
          realNext,
          credits: credits.find(c => c.userId === profile.user!.id),
          currentIsNext: current !== undefined && realNext !== undefined && current.id === realNext.id,
          currentEnded: ended,
        }
      })
      .sort((a, b) => {
        if (b.profile.user!.username > a.profile.user!.username) {
          return -1
        }

        if (b.profile.user!.username < a.profile.user!.username) {
          return 1
        }

        return 0
      })

    if (!sort) {
      return models;
    }

    const fn = sortFns[sort.column];
    if (!fn) {
      return models;
    }

    return [...models]
      .sort((a, b) => {
        const result = fn(a, b);

        if (sort.direction === 'ASC') {
          return result * -1;
        }

        return result;
      });
  })

  protected readonly _displayedColumns = computed(() => {
    const lg = this.container.lg();
    const xl = this.container.xl();
    const width = this.container.width();

    const result: Columns[] = [
      Columns.User,
      Columns.VacationLeft,
      Columns.Overtime
    ]

    if (width >= 600) {
      result.push(Columns.Current)
    }

    if (width >= 900) {
      result.push(Columns.Since, Columns.EndsWith)
    }

    if (lg) {
      result.push(Columns.TimeTracking)
    }

    if (xl) {
      result.push(Columns.Vacation)
    }

    result.push(Columns.Actions)

    return result;
  })


  trackModel: TrackByFunction<Model> = (_, p) => p.profile.user!.id

  async ngOnInit() {
    this._loading.set(true);

    await this.loadWorkTimes()
  }

  async loadWorkTimes() {
    this._loading.set(true);

    const credits = this.workTimeService
      .getVacationCreditsLeft({ until: Timestamp.fromDate(this.endOfYear), forUsers: {} })
      .then(response => {
        this._credits.set(response.results);
      })
      .catch(err => toast.error("Resturlaub konnte nicht geladen werden", { description: ConnectError.from(err).message }));

    const worktime = this.workTimeService.getWorkTime({})
      .then(response => this._workTimes.set(response.results))
      .catch(err => {
        toast.error("Arbeitszeiten konnten nicht geladen werden", { description: ConnectError.from(err).message });
      });

    Promise.all([credits, worktime])
      .finally(() => {
        this._loading.set(false);
      });
  }

  async showHistory(profile: Profile) {
    const credits = await this.workTimeService.getVacationCreditsLeft({
      analyze: true,
      forUsers: {
        userIds: [profile.user!.id],
      },
      until: Timestamp.fromDate(this.endOfYear),
    })

    this.analyze = credits.results[0].analysis || null;
  }

  delete(id: string) {
    this.workTimeService.deleteWorkTime({
      ids: [id]
    })
      .then(() => this.loadWorkTimes())
      .catch(err => toast.error(`Eintrag konnte nicht gelöscht werden: ${ConnectError.from(err).message}`))
  }

  openModal(profile: Profile) {
    this.dialog.open(SetWorktimeDialogComponent, {
      context: profile,
    })
      .closed$
      .pipe(
        filter(result => result === 'save')
      )
      .subscribe(() => this.loadWorkTimes())
  }
}
