import { lucideAlertTriangle } from '@ng-icons/lucide';
import { BrnDialogModule, BrnDialogRef } from '@spartan-ng/ui-dialog-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';

import { ChangeDetectionStrategy, Component, OnInit, inject, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import { HlmAlertModule } from '@tierklinik-dobersberg/angular/alert';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { injectOfftimeService, injectUserService } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule } from '@tierklinik-dobersberg/angular/dialog';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { Profile } from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzSelectModule } from 'ng-zorro-antd/select';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';
import { Duration } from 'src/duration';

export type VacationType = 'vacation' | 'timeoff' | '';

@Component({
  selector: 'app-createcosts',
  standalone: true,
  imports: [
    FormsModule,
    TkdRoster2Module,
    NzRadioModule,
    NzSelectModule,
    RouterModule,
    NzDatePickerModule,
    NzAvatarModule,
    HlmDialogModule,
    BrnDialogModule,
    HlmAvatarModule,
    HlmButtonModule,
    HlmInputModule,
    HlmLabelModule,
    HlmSelectModule,
    HlmAlertModule,
    HlmIconModule,
    BrnSelectModule,
  ],
  providers: provideIcons({lucideAlertTriangle}),
  templateUrl: './createcosts.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [
    `
    :host {
      @apply block w-[440px];
    }
    `
  ]
})
export class CreatecostsComponent implements OnInit {
  private readonly offTimeService = injectOfftimeService();
  private readonly usersService = injectUserService();

  protected readonly dialogRef = inject(BrnDialogRef);
  protected readonly _profiles = signal<Profile[]>([]);

  /* Models */

  protected readonly _userId = model('');
  protected readonly _comment = model('');
  protected readonly _date = model<Date | null>(null);
  protected readonly _costs = model(0);
  protected readonly _type = model<VacationType>('');

  ngOnInit() {
    this.usersService.listUsers({})
      .then(res => {
        this._profiles.set(res.users);
      })
  }

  updateCosts(costs: string) {
    const d = Duration.parseString(costs);
    if (d.seconds === 0) {
      return;
    }

    this._costs.set(d.seconds);
  }

  async save() {
    const date = this._date();
    const costs = this._costs();
    const userId = this._userId();
    const type = this._type();
    const comment = this._comment();

    if (!date) {
      return;
    }

    await this.offTimeService.addOffTimeCosts({
      addCosts: [
        {
          date: Timestamp.fromDate(date),
          costs: Duration.seconds(costs).toProto(),
          userId: userId,
          isVacation: type === 'vacation',
          comment: comment,
        }
      ]
    })

    this.dialogRef.close();
  }
}
