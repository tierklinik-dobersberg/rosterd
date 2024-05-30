import { Injectable, Signal, computed, inject, signal } from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { NavigationStart, Router } from "@angular/router";
import { PartialMessage, Timestamp } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { injectHolidayService, injectOfftimeService, injectRosterService, injectUserService } from "@tierklinik-dobersberg/angular/connect";
import { toDateString } from '@tierklinik-dobersberg/angular/utils/date';
import { AnalyzeWorkTimeResponse, ConstraintViolationList, FindOffTimeRequestsResponse, GetHolidayResponse, GetRequiredShiftsResponse, OffTimeEntry, PlannedShift, Profile, PublicHoliday, RequiredShift, Roster, SaveRosterRequest, SaveRosterResponse, WorkShift, WorkTimeAnalysis } from "@tierklinik-dobersberg/apis";
import { addDays, endOfMonth, endOfWeek, isSameDay, startOfMonth, startOfWeek } from 'date-fns';
import { toast } from 'ngx-sonner';
import { Subject, catchError, debounceTime, filter, from, map, of, switchMap } from "rxjs";

export interface ShiftState {
  // A unique ID to identify the shift.
  // TODO(ppacher): move this one to the backend since
  // we need it there as well.
  uniqueId: string;

  // The start datetime of the shift
  from: Date;

  // The end datetime of the shift
  to: Date;

  // The ID of the shift
  workShiftId: string;

  // A list of eligible user that can be assigned to this shift.
  // If a user has an OffTimeRequest for this date, the users' ID should not
  // be part of this list.
  eligibleUsers: string[];

  // A list of actually assigned users for this shift.
  assignedUsers: string[];

  // The name of the shift
  name: string;

  // The display-name of the shift (i.e. the "short-name")
  displayName: string;

  // The recommended number of assigned employees.
  // Should be set to Infinity to indicate no limit.
  staffCount: number;

  // The color of the shift.
  color: string;

  // Off-time entries/requests index by user.
  offtimes: Record<string, OffTimeEntry[]>;

  // A list of constraint violations indexed by user.
  violations: Record<string, ConstraintViolationList>;
}

export interface RosterDateState {
  // The date of the roster date stae.
  date: Date;

  // The shift state for this roster day
  shifts: ShiftState[];

  // If there's a public holiday, this is set to the holiday
  // data.
  holiday: PublicHoliday | null;
}

export interface RosterState {
  // The first date of the roster.
  from: Date;

  // The last date of the roster.
  to: Date;

  readonly: boolean;

  // The different states for the individual roster dates.
  dates: RosterDateState[];

  // All shift definitions index by shift id.
  shiftDefinitions: Map<string, WorkShift>;

  // The Compare-And-Save index to avoid race conditions when multiple
  // users are editing the same roster.
  casIndex: bigint;

  // The name of the user that approved the roster, if any.
  approvedBy: string;
}

@Injectable()
export class RosterPlannerService {
  /** Private services */
  private readonly userService = injectUserService();
  private readonly rosterService = injectRosterService();
  private readonly offTimeService = injectOfftimeService();
  private readonly holidayService = injectHolidayService();
  private readonly router = inject(Router);

  /** Used to trigger a debounced save */
  private _triggerSave = new Subject<void>();
  private _savePending = signal(false);

  /** The current planning session state. */
  private readonly _state = signal<RosterState>({
    from: new Date(),
    to: new Date(),
    dates: [],
    readonly: false,
    shiftDefinitions: new Map(),
    casIndex: BigInt(0),
    approvedBy: '',
  });

  /** All user profiles */
  private readonly _profiles = signal<Profile[]>([]);

  /** The currently selected user ID, if any */
  private readonly _selectedUser = signal<string | null>(null);

  /** Work-time analysis per user. */
  private readonly _workTimes = signal<Record<string, WorkTimeAnalysis>>({});

  /** The type of roster we're currently editing/viewing */
  private _rosterTypeName = signal('');

  /** The ID of the roster */
  private _rosterId = signal<string | null>(null);

  /** Whether or not we're currently loading and preparing a new planning session */
  private _loading = signal(true);

  private readonly _computedSaveRosterModel = computed(() => {
    const id = this._rosterId();
    const typeName = this._rosterTypeName();
    const state = this.sessionState();

    if (!id || !typeName || !state) {
      return null
    }

    const shifts: PartialMessage<PlannedShift>[] = [];

    state.dates
      .forEach(dateState => {
        dateState.shifts
          .forEach(shift => {
            shifts.push(new PlannedShift({
              assignedUserIds: shift.assignedUsers,
              from: Timestamp.fromDate(shift.from),
              to: Timestamp.fromDate(shift.to),
              workShiftId: shift.workShiftId,
            }))
          })
      })

    return new SaveRosterRequest({
      id: id,
      rosterTypeName: typeName,
      from: toDateString(state.from),
      to: toDateString(state.to),
      shifts: shifts,
      casIndex: state.casIndex,
      readMask: {
        paths: [
          'work_time_analysis',
          'roster.cas_index',
        ]
      }
    })
  })

  // Public signals

  public readonly rosterId = this._rosterId.asReadonly();

  /** A list of all available user profiles */
  public readonly profiles = this._profiles.asReadonly();

  /** A list of user profiles that are eligible to be used during this planning session */
  public readonly eligibleProfiles = computed(() => {
    const all = this.profiles();
    const state = this.sessionState();

    const result = new Set<string>();

    state.dates
      .forEach(date => date.shifts.forEach(shift => {
        shift.eligibleUsers.forEach(userId => result.add(userId))
      }));

    return Array.from(result)
      .map(id => all.find(p => p.user!.id === id)!);
  })

  /** The currently selected user, if any */
  public readonly selectedUser = this._selectedUser.asReadonly();

  /** Whether or not a user has been selected */
  public readonly hasUserSelected = computed(() => {
    const user = this._selectedUser();

    return user === null;
  })

  /** The current state of the planning session */
  public readonly sessionState = this._state.asReadonly();

  /** The last-available work-time analysis */
  public readonly workTimes = this._workTimes.asReadonly();

  /** The maximum number of shifts on a single day */
  public readonly maxShiftCount = computed(() => {
    const session = this.sessionState();
    let max = 0;

    session.dates
      .forEach(date => {
        if (date.shifts.length > max) {
          max = date.shifts.length;
        }
      })

    return max;
  })

  /** A list of all shifts to show */
  public readonly shiftIdsToShow = signal<string[]>([]);

  public readonly computedShiftsToShow = computed(() => {
    const state = this._state();
    const ids = this.shiftIdsToShow();

    if (ids.length === 0) {
      return Array.from(state.shiftDefinitions.keys())
    }

    return ids;
  })

  /** The start of the calendar */
  public readonly calendarStart = computed(() => {
    const session = this._state();

    return startOfWeek(
      startOfMonth(session.from), {
      weekStartsOn: 1
    }
    )
  })

  /** The end of the calendar. */
  public readonly calendarEnd = computed(() => {
    const session = this._state();

    return endOfWeek(
      endOfMonth(session.to), {
      weekStartsOn: 1
    }
    )
  })

  /** A list of {Date}s in the calendar view. */
  public readonly calendarDates = computed(() => {
    const start = this.calendarStart();
    const end = this.calendarEnd();

    let iter = start;
    const result: Date[] = [];

    for (iter = start; !isSameDay(iter, end); iter = addDays(iter, 1)) {
      result.push(iter);
    }

    result.push(iter);

    return result;
  })

  /** Whether or not we are currently loading and preparing a session */
  public readonly loading = this._loading.asReadonly();

  /** Whether or not saving the roster is pending */
  public readonly savePending = this._savePending.asReadonly();

  public readonly distinctShiftTypes = computed(() => {
    const state = this._state();

    const map = new Map<string, WorkShift>();

    state.dates
      .forEach(dateState => dateState.shifts.forEach(shift => {
        map.set(shift.workShiftId, state.shiftDefinitions.get(shift.workShiftId)!)
      }))

    return Array.from(map.values())
  })

  public readonly approved = computed(() => {
    const state = this._state();
    return state.approvedBy !== '';
  });

  public readonly approvedBy = computed(() => {
    const state = this._state();
    const profiles = this.profiles();

    if (!state.approvedBy) {
      return null;
    }

    return profiles.find(p => p.user!.id === state.approvedBy) || null;
  });

  constructor() {
    // Load all users once since we don't expect any changes to users
    // during a planning session.
    this.userService
      .listUsers({})
      .then(response => this._profiles.set(response.users));

    this._triggerSave
      .pipe(
        map(() => {
          this._savePending.set(true);
        }),
        debounceTime(1000),
        takeUntilDestroyed(),
        switchMap(() => {
          if (!this._savePending()) {
            return of()
          }

          return from(this.saveRoster())
        }),
        catchError(err => {
          console.error(`Failed to save roster: ${ConnectError.from(err).message}`)

          return of(undefined);
        })
      )
      .subscribe()


    this.router
      .events
      .pipe(
        takeUntilDestroyed(),
        filter(event => event instanceof NavigationStart)
      )
      .subscribe(() => {
        if (this._savePending() && !this.sessionState().readonly) {
          this.saveRoster()
            .then(() => {
              toast.success('Diestplan wurde erfolgreich gespeichert')
            })
            .catch(err => {
              toast.error('Dienstplan konnte nicht gespeichert werden ', {
                description: ConnectError.from(err).message,
              })
            });
        }
      })
  }

  /**
   * Selects a new user.
   *
   * @param id The ID of the user to select. Or null to unselect any previously selected user
   */
  public selectOrClearUser(id: string | null) {
    if (id === this._selectedUser()) {
      this._selectedUser.set(null);
    } else {
      this._selectedUser.set(id);
    }
  }

  /** Returns a computed signal that returns the RosterDateState for the given date. */
  public watchDateState(dateOrKey: Date | string): Signal<RosterDateState> {
    if (typeof dateOrKey !== 'string') {
      dateOrKey = toDateString(dateOrKey);
    }

    return computed(() => {
      const state = this._state();
      const result = state.dates.find(date => toDateString(date.date) === dateOrKey) || {
        date: new Date(),
        holiday: null,
        offtimes: [],
        shifts: []
      };

      return result
    })
  }

  /**
   * Assigns a user to a shift.
   *
   * @param userId The unique ID of the user to assign
   * @param dateKey The key of the state that contains the shift to assign the user to
   * @param uniqueShiftId The unique ID of the shift.
   */
  public assingUser(userId: string, dateKey: string, uniqueShiftId: string) {
    this.updateUserAssignment(userId, dateKey, uniqueShiftId, true)
  }

  /**
   * Unassigns a user from a shift.
   *
   * @param userId The unique ID of the user to unassign
   * @param dateKey The key of the state that contains the shift to unassign the user from
   * @param uniqueShiftId The unique ID of the shift.
   */
  public unassingUser(userId: string, dateKey: string, uniqueShiftId: string) {
    this.updateUserAssignment(userId, dateKey, uniqueShiftId, false)
  }

  /** Toggle the assignment of a upser */
  public toggleUserAssignment(userId: string, dateKey: string, uniqueShiftId: string) {
    this.updateUserAssignment(userId, dateKey, uniqueShiftId, null)
  }


  private updateUserAssignment(userId: string, dateKey: string, uniqueShiftId: string, assign: boolean | null) {
    const current = this._state();

    const assignedUsers = current.dates
      .find(d => toDateString(d.date) === dateKey)
      ?.shifts
      .find(shift => shift.uniqueId === uniqueShiftId)
      ?.assignedUsers;

    if (assignedUsers === undefined) {
      throw new Error(`Failed to find shift with id ${uniqueShiftId} for date ${dateKey}`)
    }

    const set = new Set(assignedUsers);

    if (assign) {
      set.add(userId)
    } else {
      set.delete(userId);
    }

    this.setShiftAssignments(dateKey, uniqueShiftId, Array.from(set.values()));
  }

  /** Overwrite the user assignments of a given shift. */
  public setShiftAssignments(dateKey: string, uniqueShiftId: string, userIds: string[]) {
    this._state.update(current => {
      if (!current) {
        console.error(`Cannot update user assignment: planning-session not yet prepared.`);
        return current;
      }


      const dateStateIndex = current.dates.findIndex(state => toDateString(state.date) === dateKey);
      if (dateStateIndex < 0) {
        console.error(`Failed to find RosterDateState for date key ${dateKey}`);
        return current;
      }

      const dateState = current.dates[dateStateIndex]!;
      const shiftIndex = dateState.shifts.findIndex(shift => shift.uniqueId === uniqueShiftId);
      if (shiftIndex < 0) {
        console.error(`Failed to find ShiftState for id ${uniqueShiftId}`);
        return current;
      }

      const shift = dateState.shifts[shiftIndex]!;

      // Avoid circular updates if the list is already the same.
      const previousAssignedUsers = [...dateState.shifts[shiftIndex].assignedUsers].sort();
      userIds.sort();
      if (JSON.stringify(previousAssignedUsers) === JSON.stringify(userIds)) {
        return current;
      }

      dateState.shifts = [...dateState.shifts];
      dateState.shifts[shiftIndex] = {
        ...shift,
      }


      dateState.shifts[shiftIndex].assignedUsers = Array.from(new Set(userIds));

      const updated = { ...current };
      updated.dates = [...updated.dates];
      updated.dates[dateStateIndex] = {
        ...dateState,
      }

      // Trigger saving the roster in the background.
      this._triggerSave.next();

      return updated;
    })
  }

  public reload() {
    const id = this._rosterId();
    if (!id) {
      return;
    }

    this.prepareForEdit(this._rosterTypeName(), id, this.sessionState().readonly);
  }

  /**
   * Sets the ID of the roster that is being viewed or edited.
   * When called, this loads the current roster as well as the public holidays
   * in the specified time-range and re-computes the RosterState.
   *
   * @param typeName The name of the roster type.
   * @param id The ID of the roster that is being viewed or edited.
   */
  public prepareForEdit(typeName: string, id: string, readonly = false): Promise<void> {
    this._loading.set(true);

    return this.rosterService
      .getRoster({
        rosterTypeNames: typeName ? [typeName] : undefined,
        search: {
          case: 'id',
          value: id,
        },
      })
      .then(response => {
        if (!response.roster || response.roster.length !== 1) {
          throw new Error('No or too many rosters returned by the backend');
        }

        const roster = response.roster[0];

        // extract month and year
        const [fromYear, fromMonth] = roster.from.split("-");
        const [toYear, toMonth] = roster.to.split("-")

        if (fromYear !== toYear || fromMonth !== toMonth) {
          console.error("invalid roster, only rosters for within one month are supported for now")
          return;
        }

        const fetchHolidays = this.holidayService
          .getHoliday({
            month: BigInt(+fromMonth),
            year: BigInt(+fromYear),
          })

        const requiredShifts = this.rosterService
          .getRequiredShifts({
            from: roster.from,
            to: roster.to,
            rosterTypeName: roster.rosterTypeName,
          });

        const offTimes = this.offTimeService
          .findOffTimeRequests({
            from: Timestamp.fromDate(new Date(roster.from)),
            to: Timestamp.fromDate(new Date(roster.to)),
            userIds: [],
          })

        const workTimes = this.rosterService.analyzeWorkTime({
          from: roster.from,
          to: roster.to,
          users: {
            allUsers: true
          }
        });

        return Promise.all([
          Promise.resolve(roster),
          fetchHolidays,
          requiredShifts,
          offTimes,
          workTimes,
        ])
      })
      .then(response => {
        if (!response) {
          throw new Error(`Something went wrong while loading the roster: no reponse available`)
        }

        const roster = response[0];
        const holidays = response[1];
        const requiredShifts = response[2];
        const offTimes = response[3];
        const workTimes = response[4];

        this._rosterId.set(roster.id);
        this._rosterTypeName.set(roster.rosterTypeName);

        return this._prepareState(roster, holidays, requiredShifts, offTimes, workTimes, readonly);
      })
      .catch(err => {
        this._rosterId.set(null);
        this._rosterTypeName.set('');

        const connectError = ConnectError.from(err);
        toast.error(`Failed to prepare planning session`, { description: connectError.message })

        throw connectError;
      })
      .finally(() => {
        this._loading.set(false);
      });
  }

  public toggleReadonly() {
    this._state.update(current => {
      const copy = { ...current };

      copy.readonly = !copy.readonly;

      return copy;
    })
  }

  public saveRoster(): Promise<SaveRosterResponse> {
    const saveModel = this._computedSaveRosterModel();

    if (!saveModel) {
      console.error(`Planning session not yet ready, no saveModel computed`)

      return Promise.reject(new Error('Planning session not yet ready'));
    }

    return this.rosterService
      .saveRoster(saveModel)
      .then(response => {
        this._savePending.set(false);
        this._workTimes.set(
          indexWorkTimeAnalysis(response.workTimeAnalysis)
        );

        this._state.update(current => ({
          ...current,
          casIndex: response.roster!.casIndex,
        }))

        return response;
      })
      .catch(err => {
        const connectErr = ConnectError.from(err);

        console.error(connectErr);

        if (connectErr.code === Code.FailedPrecondition) {
          toast.error('Dieser Dienstplan wurde in der Zwischenzeit bearbeitet', {
            description: connectErr.message,
          })
        }

        throw err;
      });
  }

  private _prepareState(
    roster: Roster,
    holidays: GetHolidayResponse,
    shifts: GetRequiredShiftsResponse,
    offTimes: FindOffTimeRequestsResponse,
    workTimes: AnalyzeWorkTimeResponse,
    readonly: boolean) {

    const rosterState: RosterState = {
      from: new Date(roster.from),
      to: new Date(roster.to),
      dates: [],
      readonly: readonly,
      shiftDefinitions: new Map(),
      casIndex: roster.casIndex,
      approvedBy: roster.approverUserId,
    }

    // first, create a lookup map for the public holidays
    const holidayLookupMap = new Map<string, PublicHoliday>();
    holidays.holidays
      .forEach(ph => {
        holidayLookupMap.set(ph.date, ph)
      });

    // create a lookup map for all already planned shifts
    const plannedShifts = new Map<string, PlannedShift[]>();
    roster.shifts
      .forEach(shift => {
        const dateKey = toDateString(shift.from!);
        const arr = plannedShifts.get(dateKey) || [];
        arr.push(shift);

        plannedShifts.set(dateKey, arr)
      })

    // Next, create a lookup map for all shift definitions
    shifts.workShiftDefinitions
      .forEach(def => rosterState.shiftDefinitions.set(def.id, def));

    // Now, create a date-state object for all required shifts
    const dateMap = new Map<string, RosterDateState>();

    // Create a RosterDateState entry for each date in the roster.
    let iter = rosterState.from;
    for (iter = rosterState.from; !isSameDay(iter, rosterState.to); iter = addDays(iter, 1)) {
      const key = toDateString(iter);
      dateMap.set(key, {
        date: iter,
        holiday: holidayLookupMap.get(key) || null,
        shifts: []
      })
    }

    {
      const key = toDateString(iter);
      dateMap.set(key, {
        date: iter,
        holiday: holidayLookupMap.get(key) || null,
        shifts: []
      })
    }

    shifts.requiredShifts
      .forEach(shift => {
        const dateKey = toDateString(shift.from!);

        const dateState: RosterDateState = dateMap.get(dateKey)!;
        if (!dateState) {
          throw new Error(`Missing RosterDateState entry for ${dateKey}`)
        }

        // SAFETY: we know for sure that the definition must exist!
        const definition = rosterState.shiftDefinitions.get(shift.workShiftId)!;

        const offtimes: Record<string, OffTimeEntry[]> = {};
        offTimes.results
          .filter(entry => timeOverlaps([shift.from!, shift.to!], [entry.from!, entry.to!]))
          .forEach(value => {
            const arr = offtimes[value.requestorId] || [];
            arr.push(value);

            offtimes[value.requestorId] = arr;
          })

        // SAFETY: we know for sure that .shifts is defined!
        dateState.shifts!
          .push({
            assignedUsers: plannedShifts.get(dateKey)?.find(planned => planned.workShiftId === shift.workShiftId)?.assignedUserIds || [],
            color: definition.color,
            displayName: definition.displayName,
            workShiftId: definition.id,
            name: definition.name,
            staffCount: Number(definition.requiredStaffCount),
            eligibleUsers: shift.eligibleUserIds,
            from: shift.from!.toDate(),
            to: shift.to!.toDate(),
            uniqueId: getUniqueShiftId(shift),
            offtimes: offtimes,
            violations: shift.violationsPerUserId,
          })

        dateMap.set(dateKey, dateState);
      })

    rosterState.dates = Array.from(dateMap.values())

    console.log("planning session ready: ", rosterState);

    this._state.set(rosterState);
    this._workTimes.set(indexWorkTimeAnalysis(workTimes.results));
  }
}

function timeOverlaps(a: [Timestamp, Timestamp], b: [Timestamp, Timestamp]): boolean {
  const astart = a[0].seconds;
  const aend = a[1].seconds;

  const bstart = b[0].seconds;
  const bend = b[1].seconds;

  if (bend <= astart) {
    return false;
  }

  if (bstart >= aend) {
    return false;
  }

  return true
}

export function getUniqueShiftId(shift: RequiredShift | PlannedShift): string {
  return `${shift.workShiftId}:${shift.from!.seconds}-${shift.to!.seconds}`;
}

function indexWorkTimeAnalysis(workTimes: WorkTimeAnalysis[]): Record<string, WorkTimeAnalysis> {
  const workTimesPerUser: Record<string, WorkTimeAnalysis> = {};
  workTimes
    .forEach(value => {
      workTimesPerUser[value.userId] = value;
    })

  return workTimesPerUser;
}
