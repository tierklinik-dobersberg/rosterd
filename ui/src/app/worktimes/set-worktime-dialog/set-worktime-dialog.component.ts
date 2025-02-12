import { DIALOG_DATA } from '@angular/cdk/dialog';
import { ChangeDetectionStrategy, Component, ViewChild, computed, inject, model } from "@angular/core";
import { FormGroup, FormsModule, NgForm } from '@angular/forms';
import { ConnectError } from '@connectrpc/connect';
import { BrnDialogModule, BrnDialogRef } from '@spartan-ng/ui-dialog-brain';
import { BrnSeparatorModule } from '@spartan-ng/ui-separator-brain';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmBadgeDirective } from '@tierklinik-dobersberg/angular/badge';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { HlmCheckboxModule } from '@tierklinik-dobersberg/angular/checkbox';
import { injectWorktimeSerivce } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule } from '@tierklinik-dobersberg/angular/dialog';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { DisplayNamePipe } from '@tierklinik-dobersberg/angular/pipes';
import { HlmSeparatorModule } from '@tierklinik-dobersberg/angular/separator';
import { Duration } from '@tierklinik-dobersberg/angular/utils/date';
import { SetWorkTimeRequest, WorkTime } from '@tierklinik-dobersberg/apis/roster/v1';
import { Profile  } from '@tierklinik-dobersberg/apis/idm/v1';
import { toast } from 'ngx-sonner';
import { TkdErrorMessagesComponent } from 'src/app/common/error-messages';
import { UserAvatarPipe, UserLetterPipe } from '@tierklinik-dobersberg/angular/pipes';
import { DurationValidatorDirective } from '@tierklinik-dobersberg/angular/validators';

@Component({
  standalone: true,
  imports: [
    HlmDialogModule,
    BrnDialogModule,
    HlmInputModule,
    HlmButtonModule,
    HlmCheckboxModule,
    HlmLabelModule,
    FormsModule,
    DisplayNamePipe,
    UserLetterPipe,
    HlmAvatarModule,
    BrnSeparatorModule,
    HlmSeparatorModule,
    HlmBadgeDirective,
    DurationValidatorDirective,
    TkdErrorMessagesComponent,
    UserAvatarPipe,
  ],
  providers: [],
  templateUrl: './set-worktime-dialog.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SetWorktimeDialogComponent {
  private readonly dialogRef = inject(BrnDialogRef);
  private readonly service = injectWorktimeSerivce();

  @ViewChild(NgForm, { static: true })
  protected ngForm!: FormGroup;

  protected readonly profile = inject(DIALOG_DATA) as Profile;

  protected readonly _workTimePerWeek = model<string>();
  protected readonly _vacationPerYear = model<number>();
  protected readonly _overtimeAllowance = model<string>();
  protected readonly _from = model<string>();
  protected readonly _to = model<string | null>();
  protected readonly _timeTracking = model<boolean>();

  protected readonly _computedCurrentModel = computed(() => {
    const timePerWeek = this._workTimePerWeek();
    const vacation = this._vacationPerYear();
    const overtime = this._overtimeAllowance();
    const from = this._from();
    const to = this._to();
    const timeTracking = this._timeTracking();

    if (typeof vacation !== 'number') {
      return null;
    }

    if (typeof timePerWeek !== 'string') {
      return null;
    }

    if (!this.profile.user) {
      return null;
    }

    if (!from) {
      return null;
    }

    try {
      const workTime = new WorkTime({
        userId: this.profile.user?.id,
        timePerWeek: Duration.parseString(timePerWeek).toProto(),
        applicableAfter: from,
        vacationWeeksPerYear: vacation,
        overtimeAllowancePerMonth: overtime ? Duration.parseString(overtime).toProto() : undefined,
        excludeFromTimeTracking: !timeTracking,
        endsWith: to ? to : undefined,
      })

      return workTime
    } catch (err: unknown) {
      console.error(err);

      return null;
    }
  })

  constructor() {
    // this is the default in austria
    this._vacationPerYear.set(5);

    // also, we normally want time-tracking to be enabled.
    this._timeTracking.set(true);

  }

  protected save() {
    const model = this._computedCurrentModel();
    if (!model) {
      toast.error(`Ungültige Arbeitszeit angegeben.`)
      return;
    }

    this.service
      .setWorkTime(new SetWorkTimeRequest({
        workTimes: [model],
      }))
      .then(() => {
        toast.success('Arbeitszeit wurde erfolgreich gespeichert.')
      })
      .then(() => {
        this.dialogRef.close('save')
      })
      .catch(err => {
        toast.error(`Arbeitszeit konnte nicht gespeichert werden: ${ConnectError.from(err).message}`)
      })
  }

  protected abort() {
    this.dialogRef.close('abort')
  }
}
