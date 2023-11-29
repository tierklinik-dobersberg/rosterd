import { ChangeDetectionStrategy, ChangeDetectorRef, Component, DestroyRef, OnInit, inject } from "@angular/core";
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, NavigationStart, ParamMap, Router } from "@angular/router";
import { PartialMessage, Timestamp } from "@bufbuild/protobuf";
import { GetRosterResponse, OffTimeEntry, PlannedShift, Profile, PublicHoliday, RequiredShift, Roster, SaveRosterRequest, SaveRosterResponse, WorkShift, WorkTimeAnalysis } from '@tkd/apis';
import { NzCalendarMode } from "ng-zorro-antd/calendar";
import { NzMessageService } from 'ng-zorro-antd/message';
import { Subject, forkJoin, from, of, throwError } from 'rxjs';
import { debounceTime, filter, switchMap } from "rxjs/operators";
import { HOLIDAY_SERVICE, OFFTIME_SERVICE, ROSTER_SERVICE, USER_SERVICE } from '@tkd/angular/connect';
import { toDateString } from "src/utils";

export interface RosterShift extends RequiredShift {
  definition: WorkShift;
}

@Component({
  selector: 'tkd-roster-planner',
  templateUrl: './roster-planner.html',
  styleUrls: ['./roster-planner.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class TkdRosterPlannerComponent implements OnInit {
  /* Services */
  readonly rosterService = inject(ROSTER_SERVICE);
  readonly holidayService = inject(HOLIDAY_SERVICE)
  readonly usersService = inject(USER_SERVICE);
  readonly offTimeService = inject(OFFTIME_SERVICE);
  readonly destroyRef = inject(DestroyRef);
  readonly cdr = inject(ChangeDetectorRef);

  private debounceSave$ = new Subject<void>();
  private savePending = false;

  /** The currently selected date */
  selectedDate: Date | null = null;

  /** The currently selected user from the side-bar */
  selectedUser: string | null = null;

  /** A list of all users that can be assigned */
  profiles: Profile[] = [];

  /** The id of the user for which shifts should be highlighted */
  highlightUserShifts: string | null = null;

  publicHolidays: {
    [date: string]: PublicHoliday;
  } = {}

  readonly = false;

  /** Current Roster */
  roster: PartialMessage<SaveRosterRequest> = {
    id: '',
    from: '',
    to: '',
    shifts: [],
  }

  from: Date | null = null;
  to: Date | null = null;

  shifts: {
    [date: string]: RosterShift[]
  } = {};

  plannedShifts: {
    [date: string]: PlannedShift[]
  } = {};

  workTimeByUser: {
    [userId: string]: WorkTimeAnalysis | undefined
  } = {};

  offTimeEntries: {
    [userId: string]: OffTimeEntry[] | undefined
  } = {};

  maxShifts = 0;

  dateDisabled = (d: Date | undefined) => {
    if (!d || !this.selectedDate || !this.from || !this.to) {
      return false
    }

    return d.getTime() < this.from.getTime() || d.getTime() > this.to.getTime();

    //return d.getMonth() !== this.selectedDate.getMonth();
  }

  constructor(
    private router: Router,
    private currentRoute: ActivatedRoute,
    private nzMessage: NzMessageService,
  ) { }

  rosterType: string = '';

  saveRoster() {
    return this.rosterService.saveRoster({
      id: this.roster.id,
      from: this.roster.from,
      to: this.roster.to,
      shifts: this.roster.shifts,
      rosterTypeName: this.rosterType,
      readMask: {
        paths: ['roster.id', 'work_time_analysis'],
      }
    }).catch(err => {
      return new SaveRosterResponse({
        roster: this.roster,
        workTimeAnalysis: Object.keys(this.workTimeByUser)
          .map(key => this.workTimeByUser[key])
          .filter(value => !!value) as WorkTimeAnalysis[]
      })
    })
  }

  ngOnInit(): void {
    this.router
      .events
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        filter(event => event instanceof NavigationStart),
      )
      .subscribe(async () => {
        if (this.savePending && !this.readonly) {
          await this.saveRoster()
          this.nzMessage.info("Dienstplan wurde erfolgreich gespeichert")
        }
      })

    this.debounceSave$
      .pipe(
        debounceTime(1000),
        switchMap(() => {
          return from(this.saveRoster())
        }),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe(response => {
        if (this.readonly) {
          this.nzMessage.warning("Du bist nicht berechtigt den Dienstplan zu bearbeiten")

          return;
        }

        this.savePending = false;
        if (this.roster.id !== response.roster?.id) {

        }

        this.roster.id = response.roster?.id;
        this.workTimeByUser = {};

        response.workTimeAnalysis.forEach(wt => {
          this.workTimeByUser[wt.userId] = wt;
        });

        this.cdr.markForCheck();
      })

    this.currentRoute
      .paramMap
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        switchMap(params => {
          const id = params.get("id");
          if (!!id) {
            return from(this.rosterService.getRoster({
              search: {
                case: 'id',
                value: id,
              },
            }))
          }

          return of(params);
        }),
        filter(result => {
          if (result instanceof GetRosterResponse) {
            if (result.roster.length === 0) {
              return true
            }

            return result.roster[0].id !== this.roster.id;
          }

          return true
        }),
        switchMap((params: ParamMap | GetRosterResponse) => {
          let fromDate: string;
          let toDate: string;

          if (params instanceof GetRosterResponse) {
            if (!params.roster?.length) {
              this.router.navigate(['/roster'])
            }

            this.rosterType = params.roster[0].rosterTypeName
            fromDate = params.roster[0].from
            toDate = params.roster[0].to
          } else {
            this.rosterType = params.get("type")!
            fromDate = params.get("from")!
            toDate = params.get("to")!
          }

          if (!fromDate || !toDate) {
            return throwError(() => new Error("missing year or month parameter"))
          }

          this.readonly = this.currentRoute.snapshot.data['readonly'] || false;

          this.from = new Date(fromDate)
          this.to = new Date(toDate);

          this.selectedDate = this.from;

          return forkJoin({
            from: of(fromDate),
            to: of(toDate),

            holidays: from(
              this.holidayService.getHoliday({
                month: BigInt(this.from.getMonth() + 1),
                year: BigInt(this.from.getFullYear()),
              })
            ),

            workTime: from(this.rosterService.analyzeWorkTime({
              from: fromDate,
              to: toDate,
              users: {
                allUsers: true
              }
            })),

            roster: params instanceof GetRosterResponse
              ? of(params)
              : from(this.rosterService.getRoster({
                search: {
                  case: 'date',
                  value: Timestamp.fromDate(new Date(this.from.getFullYear(), this.from.getMonth(), 2)),
                },
                rosterTypeNames: [this.rosterType],
              }).catch(err => new GetRosterResponse())),

            requiredShifts: from(this.rosterService.getRequiredShifts({
              from: fromDate,
              to: toDate,
              rosterTypeName: this.rosterType,
            })),

            users: from(this.usersService.listUsers({})),

            offTime: from(this.offTimeService.findOffTimeRequests({
              from: Timestamp.fromDate(this.from),
              to: Timestamp.fromDate(this.to),
            }))
          })
        })
      )
      .subscribe(result => {
        if (result.roster.roster.length > 1) {
          // FIXME
          result.roster.roster = [result.roster.roster[0]];
          // throw new Error("got more than one roster")
        }

        let allowedRoles = new Set<string>();
        let roster: Roster;

        if (result.roster.roster.length) {
          roster = result.roster.roster[0];
        } else {
          roster = new Roster({
            id: '',
            from: result.from,
            to: result.to,
            shifts: [],
            rosterTypeName: this.rosterType,
          })
        }

        this.roster = roster;

        this.plannedShifts = {};
        roster.shifts?.forEach(planned => {
          const key = toDateString(new Timestamp(planned.from).toDate())
          if (!this.plannedShifts[key]) {
            this.plannedShifts[key] = [];
          }

          this.plannedShifts[key].push(planned)
        })

        const shiftDefinitions = new Map<string, WorkShift>();
        result.requiredShifts.workShiftDefinitions.forEach(s => {
          // collect all allowed role ids.
          s.eligibleRoleIds
            .forEach(roleId => allowedRoles.add(roleId))

          shiftDefinitions.set(s.id, s)
        })

        this.maxShifts = 0;
        this.shifts = {};

        result.requiredShifts.requiredShifts.forEach(shift => {
          const key = toDateString(shift.from!.toDate())

          if (!this.shifts[key]) {
            this.shifts[key] = [];
          }

          Object.defineProperty(shift, 'definition', {
            writable: false,
            enumerable: true,
            value: shiftDefinitions.get(shift.workShiftId)!
          });

          this.shifts[key].push(shift as RosterShift);

          if (this.shifts[key].length > this.maxShifts) {
            this.maxShifts = this.shifts[key].length;
          }
        })

        this.profiles = result.users.users
          .filter(profile => {
            return profile.roles
              .some(role => allowedRoles.has(role.id))
          });

        this.workTimeByUser = {};
        result.workTime.results.forEach(wt => {
          this.workTimeByUser[wt.userId] = wt
        })

        this.offTimeEntries = {};
        result.offTime.results.forEach(entry => {
          if (!this.offTimeEntries[entry.requestorId]) {
            this.offTimeEntries[entry.requestorId] = [];
          }

          this.offTimeEntries[entry.requestorId]?.push(entry);
        })

        this.publicHolidays = {};
        result.holidays.holidays.forEach(ph => {
          this.publicHolidays[ph.date] = ph
        })

        this.cdr.markForCheck();
      });
  }

  setSelectedDate(date: Date) {
    // this.router.navigate(['/roster/plan/', date.getFullYear(), date.getMonth() + 1]);
  }

  setSelectedUser(username: string) {
    if (this.selectedUser === username) {
      this.selectedUser = null;
      return;
    }

    this.selectedUser = username;
  }

  setRosterShifts(date: string, shifts: PartialMessage<PlannedShift>[]) {
    shifts.forEach(updated => {
      const existingIndex = this.roster.shifts?.findIndex(s => {
        return s.workShiftId === updated.workShiftId
          && (new Timestamp(s.from).equals(new Timestamp(updated.from)))
          && (new Timestamp(s.to).equals(new Timestamp(updated.to)));
      })

      if (existingIndex !== undefined && existingIndex > -1) {
        this.roster.shifts!.splice(existingIndex, 1, updated);
      } else {
        if (!this.roster.shifts) {
          this.roster.shifts = [];
        }
        this.roster.shifts!.push(updated);
      }
    })

    this.savePending = true
    this.debounceSave$.next()
  }

  /**
   * Callback when the user selected a day in the roster
   */
  onDateSelected(date: Date): void {
    const changed = !this.selectedDate || (date.getMonth() !== this.selectedDate.getMonth() || date.getFullYear() !== this.selectedDate.getFullYear());

    if (changed) {
      this.onPanelChange({
        date,
        mode: 'month'
      });
    }
  }

  /**
   * Callback for changes in the date displayed.
   *
   * @param param0 The event emitted
   */
  onPanelChange({ date, mode }: { date: Date, mode: NzCalendarMode }): void {
    if (mode === 'year') {
      return;
    }
    this.setSelectedDate(date);
  }
}
