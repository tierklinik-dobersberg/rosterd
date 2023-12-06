import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router, RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import { Profile } from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzSelectModule } from 'ng-zorro-antd/select';
import { OFFTIME_SERVICE, USER_SERVICE } from '@tierklinik-dobersberg/angular/connect';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';
import { Duration } from 'src/duration';

interface CreateModel {
  userId: string;
  comment: string;
  date: Date;
  costs: number;
  type: 'vacation' | 'timeOff';
}

function makeEmptyCreateModel(): CreateModel {
  return {
    userId: '',
    comment: '',
    date: new Date(),
    costs: 0,
    type: 'timeOff',
  }
}

@Component({
  selector: 'app-createcosts',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    TkdRoster2Module,
    NzRadioModule,
    NzSelectModule,
    RouterModule,
    NzDatePickerModule,
    NzAvatarModule
  ],
  templateUrl: './createcosts.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class CreatecostsComponent implements OnInit {
  private readonly offTimeService = inject(OFFTIME_SERVICE);
  private readonly usersService = inject(USER_SERVICE);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly router = inject(Router);

  profiles: Profile[] = [];
  model: CreateModel = makeEmptyCreateModel();

  ngOnInit() {
    this.usersService.listUsers({})
    .then(res => {
      this.profiles = res.users;
      this.cdr.markForCheck();
    })
  }

  updateCosts(costs: string) {
    const d = Duration.parseString(costs);
    if (d.seconds === 0) {
      return;
    }

    this.model.costs = d.seconds;
  }

  async save() {
    await this.offTimeService.addOffTimeCosts({
      addCosts: [
        {
          date: Timestamp.fromDate(this.model.date),
          costs: Duration.seconds(this.model.costs).toProto(),
          userId: this.model.userId,
          isVacation: this.model.type === 'vacation',
          comment: this.model.comment,
        }
      ]
    })

    this.router.navigate(['/costs'])
  }
}
