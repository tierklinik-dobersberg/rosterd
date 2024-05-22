import { ChangeDetectionStrategy, Component, OnInit, TrackByFunction, computed, inject, model, signal } from '@angular/core';
import { RouterModule } from '@angular/router';
import { ConnectError } from '@connectrpc/connect';
import { lucideCopy, lucideListPlus, lucideMoreVertical, lucidePencil, lucideTrash2 } from '@ng-icons/lucide';
import { BrnAlertDialogModule } from '@spartan-ng/ui-alertdialog-brain';
import { BrnDialogModule } from '@spartan-ng/ui-dialog-brain';
import { BrnMenuTriggerDirective } from '@spartan-ng/ui-menu-brain';
import { BrnTableModule } from '@spartan-ng/ui-table-brain';
import { HlmAlertDialogModule } from '@tierklinik-dobersberg/angular/alertdialog';
import { HlmBadgeModule } from '@tierklinik-dobersberg/angular/badge';
import { HlmButtonDirective } from '@tierklinik-dobersberg/angular/button';
import { injectRoleService, injectWorkShiftService } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule, HlmDialogService } from '@tierklinik-dobersberg/angular/dialog';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmMenuModule } from '@tierklinik-dobersberg/angular/menu';
import { DaytimePipe, RoleListPipe, WorkDayPipe } from '@tierklinik-dobersberg/angular/pipes';
import { HlmSpinnerModule } from '@tierklinik-dobersberg/angular/spinner';
import { HlmTableModule } from '@tierklinik-dobersberg/angular/table';
import { CreateWorkShiftRequest, Role, WorkShift } from '@tierklinik-dobersberg/apis';
import { NzDropDownModule } from 'ng-zorro-antd/dropdown';
import { NzModalModule } from 'ng-zorro-antd/modal';
import { toast } from 'ngx-sonner';
import { filter } from 'rxjs';
import { sortProtoDaytime, sortProtoDuration } from '../common/behaviors';
import { injectContainerSize } from '../common/container';
import { TkdRoster2Module } from '../roster2/roster2.module';
import { SortColumn, TkdTableSortColumnComponent } from './../common/table-sort/table-sort.component';
import { WorkshiftsComponent } from './workshifts/workshifts.component';

enum Columns {
  Name = 'name',
  DisplayName = 'displayName',
  Type = 'type',
  Weekdays = 'weekdays',
  Description = 'description',
  Time = 'time',
  Duration = 'duration',
  Value = 'value',
  Tags = 'tags',
  EmployeeCount = 'employeeCount',
  EligibleRoles = 'eligibleRoles',
  Actions = 'actions'
}

type SortFunc = (a: WorkShift, b: WorkShift) => number;

const sortFns: {
  [key in Columns]?: SortFunc
} = {
  [Columns.Name]: (a, b) => b.name.localeCompare(a.name),
  [Columns.DisplayName]: (a, b) => b.displayName.localeCompare(a.displayName),
  [Columns.Type]: (a, b) => {
    const an = a.onHoliday ? 1 : 0;
    const bn = b.onHoliday ? 1 : 0;
    return bn - an;
  },
  [Columns.Time]: (a, b) => sortProtoDaytime(a.from, b.from),
  [Columns.Value]: (a, b) => sortProtoDuration(b.timeWorth, a.timeWorth),
  [Columns.EmployeeCount]: (a, b) => Number(b.requiredStaffCount || 0) - Number(a.requiredStaffCount || 0),
} as const;

@Component({
  standalone: true,
  imports: [
    RouterModule,
    TkdRoster2Module,
    DaytimePipe,
    WorkDayPipe,
    RoleListPipe,
    NzDropDownModule,
    NzModalModule,
    HlmButtonDirective,
    HlmMenuModule,
    HlmIconModule,
    BrnMenuTriggerDirective,
    HlmAlertDialogModule,
    BrnAlertDialogModule,
    HlmTableModule,
    BrnTableModule,
    HlmBadgeModule,
    TkdTableSortColumnComponent,
    HlmSpinnerModule,
    HlmDialogModule,
    BrnDialogModule,
  ],
  templateUrl: './admin.component.html',
  providers: provideIcons({
    lucideMoreVertical,
    lucidePencil,
    lucideTrash2,
    lucideCopy,
    lucideListPlus
  }),
  styles: [
    `
    :host {
      @apply flex flex-col overflow-hidden flex-grow;
    }
    `
  ],
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminComponent implements OnInit {
  private readonly workShiftService = injectWorkShiftService();
  private readonly roleService = injectRoleService();
  private readonly dialog = inject(HlmDialogService);

  // Signals and template variables
  protected readonly container = injectContainerSize();
  protected readonly columns = Columns;

  protected readonly _loading = signal(false);
  protected readonly _workshifts = signal<WorkShift[]>([]);
  protected readonly _roles = signal<Role[]>([]);
  protected readonly _sort = model<SortColumn<typeof sortFns> | null>(null);

  protected readonly _sortedShifts = computed(() => {
    const shifts = this._workshifts();
    const sort = this._sort();

    if (!sort) {
      return shifts;
    }

    const fn = sortFns[sort.column];
    if (!fn) {
      return shifts;
    }

    return [...shifts]
      .sort((a, b) => {
        const result = fn(a, b);

        if (sort.direction === 'ASC') {
          return result * -1
        }

        return result;
      })
  })

  protected readonly _displayedColumns = computed(() => {
    const md = this.container.md();
    const lg = this.container.lg();
    const xl = this.container.xl();
    const xxl = this.container.xxl();

    const result: Columns[] = [
      Columns.Name,
      Columns.Type,
      Columns.Time,
      Columns.Duration
    ]

    if (md) {
      result.push(Columns.Weekdays, Columns.Value)
    }

    if (lg) {
      result.push(Columns.EligibleRoles)
    }

    if (xl) {
      result.push(Columns.EmployeeCount, Columns.DisplayName, Columns.Tags)
    }

    if (xxl) {
      result.push(Columns.Description)
    }

    result.push(Columns.Actions)

    return result;
  })

  protected readonly trackWs: TrackByFunction<WorkShift> = (_, w) => w.id;

  ngOnInit() {
    this.load()
  }

  private load() {
    this._loading.set(true);

    const rolePromise = this.roleService
      .listRoles({})
      .then(response => response.roles)
      .then(roles => this._roles.set(roles))
      .catch(err => toast.error(`Failed to load roles: ${ConnectError.from(err).message}`));

    const shiftPromise = this.workShiftService
      .listWorkShifts({})
      .then(response => response.workShifts)
      .then(shifts => shifts.sort((a, b) => {
        const diff = Number(a.order - b.order);
        if (diff !== 0) {
          return diff;
        }

        if (a.name > b.name) {
          return 1
        }
        if (a.name < b.name) {
          return -1
        }

        return 0
      }))
      .then(shifts => {
        shifts.forEach(shift => shift.tags.sort())
        return shifts;
      })
      .then(shifts => this._workshifts.set(shifts))
      .catch(err => toast.error(`Failed to load work-shifts: ${ConnectError.from(err).message}`));

    Promise.all([rolePromise, shiftPromise])
      .finally(() => this._loading.set(false));
  }

  protected delete(entry: WorkShift) {
    this.workShiftService
      .deleteWorkShift({ id: entry.id })
      .then(() => toast.success('Schicht wurde erfolgreich gelöscht.'))
      .then(() => this.load())
      .catch(err => {
        toast.error('Failed to delete workshift: ' + ConnectError.from(err).rawMessage)
      })
  }

  protected duplicate(entry: WorkShift) {
    const copy = new CreateWorkShiftRequest(entry)

    copy.name += " (Copy)"
    copy.order++;

    this.workShiftService
      .createWorkShift({
        ...copy,
      })
      .then(() => toast.success('Schicht wurde erfolgreich kopiert'))
      .then(() => this.load())
      .catch(err => {
        toast.error('Failed to copy workshift: ' + ConnectError.from(err).rawMessage)
      })
  }

  protected editOrCreate(shift?: WorkShift) {
    this.dialog
      .open(WorkshiftsComponent, {
        context: shift,
        contentClass: 'max-w-[unset]',
        closeOnBackdropClick: false,
        closeOnOutsidePointerEvents: false,
      })
      .closed$
      .pipe(
        filter(response => typeof response === 'string'),
      )
      .subscribe(() => this.load());
  }
}
