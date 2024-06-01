import { ChangeDetectionStrategy, Component, Injector, IterableDiffers, OnInit, computed, effect, inject, input, signal } from "@angular/core";
import { LayoutService } from "@tierklinik-dobersberg/angular/layout";
import { toDateString } from "src/utils";
import { RosterDateState, RosterPlannerService, ShiftState } from "../planner.service";

@Component({
  // eslint-disable-next-line @angular-eslint/component-selector
  selector: 'tkd-roster-planner-day',
  templateUrl: './planner-day.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  // eslint-disable-next-line @angular-eslint/no-host-metadata-property
  host: {
    '[class]': '_computedHostClass()',
  },
  styles: [
    `
    :host {
      display: flex;
      flex-direction: column;
    }
    `
  ],
})
export class TkdRosterPlannerDayComponent implements OnInit {
  protected readonly _service = inject(RosterPlannerService);
  private readonly _injector = inject(Injector);
  private readonly _differs = inject(IterableDiffers);

  /** Inputs */
  public date = input.required<string, Date | string>({
    transform: (value) => {
      if (value instanceof Date) {
        return toDateString(value);
      }

      return value as string;
    }
  })

  /** Private signals */
  private _state = signal<RosterDateState>({
    date: new Date(),
    holiday: null,
    shifts: [],
  });

  /** Template signals and variables */
  protected readonly layout = inject(LayoutService);
  protected readonly _selectedUser = computed(() => this._service.selectedUser());
  protected readonly _holiday = computed(() => this._state().holiday);
  protected readonly _shifts = computed(() => this._state().shifts);
  protected readonly _readonly = signal(true);
  protected readonly _profiles = computed(() => this._service.profiles());
  protected readonly _profileIds = computed(() => this._service.profiles().map(p => p.user!.id));
  protected readonly _workTimes = computed(() => this._service.workTimes());
  protected readonly _loading = computed(() => this._service.loading());
  protected readonly _shiftsToShow = computed(() => this._service.computedShiftsToShow());
  protected readonly _showAllUsers = computed(() => this._service.showAllUsers())

  protected readonly _isToday = computed(() => {
    const loading = this._loading();
    const date = this.date();
    const today = toDateString(new Date());

    if (loading) {
      return false;
    }

    return date === today;
  })

  protected readonly _computedHostClass = computed(() => {
    const isToday = this._isToday();

    if (isToday) {
      return 'ring ring-1 ring-emerald-600 drop-shadow-lg lg:drop-shadow-none';
    }

    return '';
  })

  protected updateShiftAssignments(shift: ShiftState, users: string[]) {
    const diff = this._differs.find([]).create<string>()
    diff.diff(shift.assignedUsers);

    const change = diff.diff(users);
    change?.forEachAddedItem(item => {
      this._service.pushToUndoStack({
        type: 'assign',
        dateKey: this.date(),
        shiftId: shift.uniqueId,
        userId: item.item
      })
    })
    change?.forEachRemovedItem(item => {
      this._service.pushToUndoStack({
        type: 'unassign',
        dateKey: this.date(),
        shiftId: shift.uniqueId,
        userId: item.item
      })
    })

    this._service.setShiftAssignments(this.date(), shift.uniqueId, users)
  }

  ngOnInit(): void {
    const date = this.date();
    const stateSignal = this._service.watchDateState(date);

    effect(() => {
      const dateState = stateSignal();
      const session = this._service.sessionState();

      this._state.set(dateState);
      this._readonly.set(session.readonly);
    }, {
      allowSignalWrites: true,
      injector: this._injector,
    })
  }
}
