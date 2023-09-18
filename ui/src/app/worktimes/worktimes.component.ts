import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, TrackByFunction, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ConnectError } from '@bufbuild/connect';
import { Timestamp } from '@bufbuild/protobuf';
import { AnalyzeVacation, Profile, UserVacationSum, WorkTime } from '@tkd/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzMessageModule, NzMessageService } from 'ng-zorro-antd/message';
import { NzModalModule } from 'ng-zorro-antd/modal';
import { NzTimelineModule } from 'ng-zorro-antd/timeline';
import { Duration } from 'src/duration';
import { toDateString } from 'src/utils';
import { USER_SERVICE, WORKTIME_SERVICE } from '../connect_clients';
import { TkdRoster2Module } from '../roster2/roster2.module';

interface Model {
  profile: Profile;
  current?: WorkTime;
  next?: WorkTime;
  credits?: UserVacationSum;
}

interface ChangeModel {
  userId: string;
  workTimePerWeek: string;
  vacationPerYear: number;
  applicableAfter: string;
}

function makeEmptyChangeModel(): ChangeModel {
  return {
    userId: '',
    workTimePerWeek: '0',
    vacationPerYear: 0,
    applicableAfter: toDateString(new Date()),
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
    NzTimelineModule,
  ],
  templateUrl: './worktimes.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class WorktimesComponent implements OnInit {
  private readonly userService = inject(USER_SERVICE)
  private readonly workTimeService = inject(WORKTIME_SERVICE);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly message = inject(NzMessageService);

  models: Model[] = [];

  parseDate = Duration.parseString;
  toTimestamp = (val: string) => Timestamp.fromDate(new Date(val));

  setForUser: Profile | null = null;
  changeModel = makeEmptyChangeModel();
  analyze: AnalyzeVacation | null = null;

  endOfYear: Date = (() => {
      const now = new Date();
      return new Date(now.getFullYear() +1, 0, 0);
  })()

  trackModel: TrackByFunction<Model> = (_, p) => p.profile.user!.id

  async ngOnInit() {
    await this.loadWorkTimes()
  }

  async loadWorkTimes() {
    try {
      const profiles = await this.userService.listUsers({}).then(response => response.users);
      const workTimes = await this.workTimeService.getWorkTime({}).then(response => response.results);

      const credits = await this.workTimeService.getVacationCreditsLeft({ until: Timestamp.fromDate(this.endOfYear) })
        .then(response => response.results)

      this.models = profiles.map(profile => {
        const wt = workTimes.find(wt => wt.userId === profile.user!.id);

        return {
          profile: profile,
          current: wt?.current,
          next: wt?.history.find(h => h.applicableAfter!.seconds > (wt.current?.applicableAfter?.seconds || 0)),
          credits: credits.find(c => c.userId === profile.user!.id),
        }
      })

      this.cdr.markForCheck();
    } catch (err) {
      console.error(err);
      debugger;

      this.message.error(ConnectError.from(err).rawMessage)
    }
  }

  async showHistory(profile:Profile) {
    const credits = await this.workTimeService.getVacationCreditsLeft({
      analyze: true,
      forUsers: {
        userIds: [profile.user!.id],
      },
      until: Timestamp.fromDate(this.endOfYear),
    })

    this.analyze = credits.results[0].analysis || null;
    this.cdr.markForCheck();
  }

  async openModal(profile: Profile) {
    this.changeModel = makeEmptyChangeModel()
    this.changeModel.userId = profile.user!.id;
    this.setForUser = profile;
  }

  async saveForUser() {
    if (!this.setForUser) {
      return;
    }

    await this.workTimeService.setWorkTime({
      workTimes: [
        {
          applicableAfter: Timestamp.fromDate(new Date(this.changeModel.applicableAfter)),
          timePerWeek: Duration.parseString(this.changeModel.workTimePerWeek).toProto(),
          userId: this.changeModel.userId,
          vacationWeeksPerYear: this.changeModel.vacationPerYear,
        }
      ]
    })

    this.setForUser = null;

    await this.loadWorkTimes()
  }
}
