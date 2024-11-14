import { lucideAlertTriangle } from '@ng-icons/lucide';
import { BrnDialogModule, BrnDialogRef } from '@spartan-ng/ui-dialog-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';

import { ChangeDetectionStrategy, Component, inject, model } from '@angular/core';
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
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { injectUserProfiles } from '@tierklinik-dobersberg/angular/behaviors';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';
import { Duration } from 'src/duration';
import { UserAvatarPipe } from '@tierklinik-dobersberg/angular/pipes';
import { DurationValidatorDirective } from '@tierklinik-dobersberg/angular/validators';
import { TkdErrorMessagesComponent } from 'src/app/common/error-messages';

export type VacationType = 'vacation' | 'timeoff' | '';

@Component({
  selector: 'app-createcosts',
  standalone: true,
  imports: [
    FormsModule,
    TkdRoster2Module,
    RouterModule,
    NzDatePickerModule,
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
    UserAvatarPipe,
    DurationValidatorDirective,
    TkdErrorMessagesComponent,
  ],
  providers: provideIcons({ lucideAlertTriangle }),
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
export class CreatecostsComponent {
  private readonly offTimeService = injectOfftimeService();
  private readonly usersService = injectUserService();

  protected readonly dialogRef = inject(BrnDialogRef);
  protected readonly _profiles = injectUserProfiles();

  /* Models */

  protected readonly _userId = model('');
  protected readonly _comment = model('');
  protected readonly _date = model<Date | null>(null);
  protected readonly _costs = model(0);
  protected readonly _type = model<VacationType>('');

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
