import { ChangeDetectionStrategy, ChangeDetectorRef, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { PartialMessage, Timestamp } from '@bufbuild/protobuf';
import { injectOfftimeService, injectRosterService, injectUserService } from '@tierklinik-dobersberg/angular/connect';
import { ApproveRosterWorkTimeSplit, OffTimeEntry, Profile, Roster, WorkTimeAnalysis } from '@tierklinik-dobersberg/apis';
import { from, switchMap } from 'rxjs';
import { Duration } from 'src/duration';

@Component({
  selector: 'app-approval',
  templateUrl: './approval.component.html',
  styles: [
    `
    :host {
      @apply block overflow-auto;
    }
    `
  ],
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class ApprovalComponent implements OnInit {
  private readonly rosterService = injectRosterService();
  private readonly offTimeService = injectOfftimeService();
  private readonly userService = injectUserService();
  private readonly currentRoute = inject(ActivatedRoute);
  private readonly destroyRef = inject(DestroyRef);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly router = inject(Router);

  profiles: Profile[] = [];

  roster: Roster | null = null;
  timeAnalysis: WorkTimeAnalysis[] = [];
  offTimePerUser: {[userId: string]: (undefined | OffTimeEntry[])} = {};

  vacationPerUser: {
    [userId: string]: number
  } = {};

  num = Number;

  updateSplit(userId: string, value: string) {
    try {
      const wt = this.timeAnalysis.find(wt => wt.userId === userId);
      if (!wt) {
        return
      }

      if (!wt.overtime || wt.overtime!.seconds >= 0) {
        return
      }

      const d = Duration.parseString(value)

      this.vacationPerUser[userId] = d.seconds;

    } catch (err) {
      console.error(err)
    }
  }

  async approve() {
    const wts: {[userId: string]: PartialMessage<ApproveRosterWorkTimeSplit>} = {};

    Object.keys(this.vacationPerUser)
      .forEach(userId => {
        const wt = this.timeAnalysis.find(wt => wt.userId === userId);

        if (!wt || !wt.overtime || wt.overtime.seconds >= 0) {
          return
        }

        const vacation = this.vacationPerUser[userId];
        const timeoff = Number(wt.overtime.seconds) + vacation;

        wts[userId] = {
          userId,
          vacation: Duration.seconds(-1 * vacation).toProto(),
          timeOff: Duration.seconds(timeoff).toProto(),
        }
      })

    await this.rosterService.approveRoster({
      id: this.roster!.id,
      workTimeSplit: wts,
    })

    this.router.navigate(['/roster'])
  }

  ngOnInit() {
    this.userService.listUsers({})
      .then(response => {
        this.profiles = response.users;
        this.cdr.markForCheck();
      });

    this.currentRoute
      .paramMap
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        switchMap(params => {
          return from(this.rosterService.getRoster({
            search: {
              case: 'id',
              value: params.get("id")!,
            },
            timeTrackingOnly: true,
          }))
        })
      )
      .subscribe(response => {
        this.roster = response.roster[0];
        this.timeAnalysis = response.workTimeAnalysis;

        this.vacationPerUser = {};
        this.timeAnalysis.forEach(wt => {
          if (!wt.overtime || Number(wt.overtime.seconds) === 0) {
            return
          }

          this.vacationPerUser[wt.userId] = 0
        })

        this.offTimeService.findOffTimeRequests({
            from: Timestamp.fromDate(new Date(this.roster.from)),
            to: Timestamp.fromDate(new Date(this.roster.from)),
          })
          .then(response => {
            this.offTimePerUser = {}
            response.results.forEach(entry => {
              const arr = this.offTimePerUser[entry.requestorId] || [];
              arr.push(entry)

              this.offTimePerUser[entry.requestorId] = arr;
            })

            this.cdr.markForCheck();
          })

        this.cdr.markForCheck();
      })
  }
}
