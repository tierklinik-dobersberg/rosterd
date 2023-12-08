import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, TrackByFunction, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { OffTimeCosts, Profile, UserOffTimeCosts } from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzSelectModule } from 'ng-zorro-antd/select';
import { OFFTIME_SERVICE, USER_SERVICE } from '@tierklinik-dobersberg/angular/connect';
import { TkdRoster2Module } from '../roster2/roster2.module';
import { NzModalModule, NzModalService } from 'ng-zorro-antd/modal';
import { NzMessageModule, NzMessageService } from 'ng-zorro-antd/message';
import { ConnectError } from '@connectrpc/connect';

@Component({
  selector: 'app-offtimecosts',
  standalone: true,
  imports: [
    CommonModule,
    NzDatePickerModule,
    NzRadioModule,
    NzAvatarModule,
    NzSelectModule,
    NzModalModule,
    NzMessageModule,
    FormsModule,
    RouterModule,
    TkdRoster2Module,
  ],
  templateUrl: './offtimecosts.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class OfftimecostsComponent implements OnInit {
  private readonly offTimeService = inject(OFFTIME_SERVICE);
  private readonly usersService = inject(USER_SERVICE);
  private readonly nzModal = inject(NzModalService)
  private readonly nzMessageService = inject(NzMessageService);
  private readonly cdr = inject(ChangeDetectorRef);

  timeRange: [Date, Date] | null = null;
  filterType: 'all' | 'vacation' | 'za' = 'all';
  filterByUser: string[] = [];

  profiles: Profile[] = [];
  allCosts: UserOffTimeCosts[] = [];
  costs: UserOffTimeCosts[] = [];

  trackUserCosts: TrackByFunction<UserOffTimeCosts> = (_, u) => u.userId;
  trackCosts: TrackByFunction<OffTimeCosts> = (_, o) => o.id;

  ngOnInit(): void {
    this.usersService.listUsers({})
      .then(res => {
        this.profiles = res.users;
        this.cdr.markForCheck();
      })

    this.loadCosts()
  }

  filterCosts() {
    this.costs = this.allCosts
      .map(c => {
        const copy = new UserOffTimeCosts(c)
        copy.costs = copy.costs.filter(cost => {
          if (this.timeRange) {
            console.log(cost)
            const dateSeconds = cost.date?.toDate().getTime() || 0;

            console.log(dateSeconds, this.timeRange.map(v => v.getTime()))
            if (!!this.timeRange[0] && this.timeRange[0].getTime() > dateSeconds) {
              return false;
            }

            if (!!this.timeRange[1] && this.timeRange[1].getTime() < dateSeconds) {
              return false;
            }
          }

          if (this.filterType !== 'all') {
            switch (this.filterType) {
              case 'vacation':
                if (!cost.isVacation) {
                  return false
                }
                break;
              case 'za':
                if (cost.isVacation) {
                  return false;
                }
                break;
            }
          }

          return true
        })

        return copy
      })
      .filter(c => {
        if (this.filterByUser.length > 0) {
          if (!this.filterByUser.includes(c.userId)) {
            return false;
          }
        }

        return c.costs.length
      })
  }

  private loadCosts() {
    this.offTimeService.getOffTimeCosts({})
      .then(res => {
        this.allCosts = res.results
        this.filterCosts()
        this.cdr.markForCheck();
      })
  }

  async delete(id: string) {
    this.nzModal
      .confirm({
        nzTitle: 'Bestätigung erforderlich',
        nzContent: 'Möchtest du den Eintrag wirklich löschen?',
        nzOkDanger: true,
        nzOkText: 'Löschen',
        nzCancelText: 'Abbrechen',
        nzOnOk: async () => {
          await this.offTimeService.deleteOffTimeCosts({ids: [id]})
            .catch(err => {
              this.nzMessageService.error('Eintrag konnte nicht gelöscht werden: ' + ConnectError.from(err).rawMessage)
            })

          this.loadCosts()
        }
      })
  }
}
