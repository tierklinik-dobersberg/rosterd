import { AfterRenderPhase, ChangeDetectionStrategy, Component, ElementRef, Injector, ViewChild, afterNextRender, computed, inject, model } from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { ActivatedRoute, Router } from "@angular/router";
import { HlmDialogService } from "@tierklinik-dobersberg/angular/dialog";
import { LayoutService } from "@tierklinik-dobersberg/angular/layout";
import { differenceInCalendarDays } from 'date-fns';
import { filter, map } from "rxjs";
import { toDateString } from "src/utils";
import { ApprovalComponent } from "../approval/approval.component";
import { RosterPlannerService } from "./planner.service";
import { TkdRosterPlannerSettingsComponent } from "./settings";


@Component({
  // eslint-disable-next-line @angular-eslint/component-selector
  selector: 'tkd-roster-planner',
  templateUrl: './roster-planner.html',
  styleUrls: ['./roster-planner.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    RosterPlannerService,
  ]
})
export class TkdRosterPlannerComponent {
  private readonly activatedRoute = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly element: ElementRef<HTMLElement> = inject(ElementRef);
  private readonly dialog = inject(HlmDialogService);

  protected readonly _service = inject(RosterPlannerService);
  protected readonly layout = inject(LayoutService);

  /** The current state */
  protected readonly _state = computed(() => this._service.sessionState());

  /** The start date of the roster */
  protected readonly _from = computed(() => this._state().from);

  /** The end date of the roster */
  protected readonly _to = computed(() => this._state().to);

  /** Whether or not we are in "readonly" mode. */
  protected readonly _readonly = computed(() => this._state().readonly);

  /** Whether or not all users should be shown */
  protected readonly _showAllUsers = model<boolean>(false);

  /** A list of eligible profiles */
  protected readonly _profiles = computed(() => {
    const eligible = this._service.eligibleProfiles()
    const all = this._service.profiles();

    if (this._showAllUsers()) {
      return all
    }

    return eligible
  });

  /** The currently selected user ID, if any */
  protected readonly _selectedUser = computed(() => this._service.selectedUser());

  /** The latest work-time calculations */
  protected readonly _workTimes = computed(() => this._service.workTimes());

  /** A list of date objects for the whole roster.
   * Used to render the day-cells in the mobile view */
  protected readonly _dates = computed(() => this._service.calendarDates());

  /** The star tof the calendar */
  protected readonly _calendarStart = computed(() => this._service.calendarStart());

  /** The end of the calendar */
  protected readonly _calendarEnd = computed(() => this._service.calendarEnd());

  /** Wether or not we are currently loading and preparing the session */
  protected readonly _loading = computed(() => this._service.loading());

  protected readonly _dirty = computed(() => this._service.savePending());

  protected readonly trackDate = (d: Date) => toDateString(d)

  @ViewChild(TkdRosterPlannerSettingsComponent, { static: false })
  protected settings?: TkdRosterPlannerSettingsComponent;

  protected print() {
    setTimeout(() => window.print(), 500);
  }

  protected close() {
    this.router.navigate(['/roster']);
  }

  protected approve() {
    if (this._dirty()) {
      return
    }

    this.dialog.open(ApprovalComponent, {
      contentClass: 'w-fit max-w-[90vw]',
      context: {
        id: this._service.rosterId(),
      },
      closeOnBackdropClick: false,
      closeOnOutsidePointerEvents: false,
    })
      .closed$
      .pipe(filter((v: string) => v === 'approve'))
      .subscribe(() => {
        this._service.reload();
      })
  }

  constructor() {
    const injector = inject(Injector);

    this.activatedRoute
      .paramMap
      .pipe(
        takeUntilDestroyed(),
        map(params => [params.get('type'), params.get('id')])
      )
      .subscribe(([type, id]) => {
        if (!id) {
          this.router.navigate(['../'])
          return
        }

        this._service.prepareForEdit(type || '', id, this.activatedRoute.snapshot.data?.readonly)
          .then(() => {
            if (!this.layout.lg()) {
              const today = toDateString(new Date());
              afterNextRender(() => {
                this.scrollToDate(today);
              }, {
                phase: AfterRenderPhase.Read,
                injector,
              })
            }
          })
      })
  }

  protected scrollToDate(date: Date | string) {
    if (date instanceof Date) {
      date = toDateString(date);
    }

    this.element.nativeElement
      .querySelector(`#date-${date} `)
      ?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
        inline: 'nearest'
      })
  }

  dateDisabled = (d: Date | undefined) => {
    if (!d) {
      return false
    }

    const isBefore = differenceInCalendarDays(d, this._from()) < 0;
    const isAfter = differenceInCalendarDays(d, this._to()) > 0;

    return isBefore || isAfter;
  }
}
