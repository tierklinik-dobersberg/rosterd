import { DIALOG_DATA } from '@angular/cdk/dialog';
import { ChangeDetectionStrategy, Component, OnInit, computed, effect, inject, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PartialMessage } from '@bufbuild/protobuf';
import { ConnectError } from '@connectrpc/connect';
import { lucideXCircle } from '@ng-icons/lucide';
import { BrnDialogModule, BrnDialogRef } from '@spartan-ng/ui-dialog-brain';
import { BrnPopoverModule } from '@spartan-ng/ui-popover-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { BrnSeparatorModule } from '@spartan-ng/ui-separator-brain';
import { HlmBadgeModule } from '@tierklinik-dobersberg/angular/badge';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { HlmCheckboxModule } from '@tierklinik-dobersberg/angular/checkbox';
import { injectRoleService, injectWorkShiftService } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule } from '@tierklinik-dobersberg/angular/dialog';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { DaytimePipe, DurationPipe } from '@tierklinik-dobersberg/angular/pipes';
import { HlmPopoverModule } from '@tierklinik-dobersberg/angular/popover';
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { HlmSeparatorModule } from '@tierklinik-dobersberg/angular/separator';
import { getDaySeconds } from '@tierklinik-dobersberg/angular/utils/date';
import { Role } from '@tierklinik-dobersberg/apis/idm/v1';
import { CreateWorkShiftRequest, Daytime, UpdateWorkShiftRequest, WorkShift } from '@tierklinik-dobersberg/apis/roster/v1';
import { toast } from 'ngx-sonner';
import { TkdErrorMessagesComponent } from 'src/app/common/error-messages';
import { DurationValidatorDirective } from '@tierklinik-dobersberg/angular/validators';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';
import { Duration } from 'src/duration';


@Component({
  standalone: true,
  imports: [
    FormsModule,
    TkdRoster2Module,
    DaytimePipe,
    DurationPipe,
    DurationValidatorDirective,

    HlmInputModule,
    HlmButtonModule,
    HlmLabelModule,
    HlmBadgeModule,
    HlmSelectModule,
    BrnSelectModule,
    HlmInputModule,
    TkdErrorMessagesComponent,
    HlmPopoverModule,
    BrnPopoverModule,
    HlmDialogModule,
    HlmCheckboxModule,
    BrnDialogModule,
    BrnSeparatorModule,
    HlmSeparatorModule,
    HlmIconModule,
  ],
  providers: [
    ...provideIcons({lucideXCircle})
  ],
  templateUrl: './workshifts.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class WorkshiftsComponent implements OnInit {
  private readonly workShiftService = injectWorkShiftService();
  private readonly roleService = injectRoleService();
  private readonly dialogRef = inject(BrnDialogRef);
  private readonly dialogData: WorkShift | undefined = inject(DIALOG_DATA, { optional: true });

  protected readonly _days = model<number[]>([]);
  protected readonly _color = model('');
  protected readonly _description = model('');
  protected readonly _duration = model('');
  protected readonly _displayName = model('');
  protected readonly _form = model('');
  protected readonly _order = model(0);
  protected readonly _staffCount = model(0);
  protected readonly _tags = model<string[]>([]);
  protected readonly _timeValue = model('');
  protected readonly _eligibleRoles = model<string[]>([]);
  protected readonly _name = model('');
  protected readonly _onHoliday = model(false);
  protected readonly _updateInPlace = model(false);
  protected readonly _roles = signal<Role[]>([]);

  private assignExisting() {
    if (!this.dialogData || !this.dialogData.id) {
      return;
    }

    const ws = this.dialogData;
    this._days.set(ws.days)
    this._color.set(ws.color);
    this._description.set(ws.description);
    this._duration.set(
      Duration.seconds(getDaySeconds(ws.duration!)).format('default-hours')
    )
    this._displayName.set(ws.displayName)
    this._form.set(new DaytimePipe().transform(ws.from));
    this._order.set(Number(ws.order))
    this._staffCount.set(Number(ws.requiredStaffCount))
    this._tags.set(ws.tags);
    this._timeValue.set(
      (ws.timeWorth && getDaySeconds(ws.timeWorth) > 0)
        ? Duration.seconds(getDaySeconds(ws.timeWorth)).format('default-hours')
        : ''
    )
    this._eligibleRoles.set(ws.eligibleRoleIds)
    this._name.set(ws.name)
    this._onHoliday.set(ws.onHoliday)
  }

  protected readonly _isEdit = signal(this.dialogData && this.dialogData.id);

  protected readonly _computedCurrentModel = computed(() => {
    const [hour, minute] = this._form().split(":");

    const common: Partial<PartialMessage<UpdateWorkShiftRequest> & PartialMessage<CreateWorkShiftRequest>> = {
      days: this._days(),
      color: this._color(),
      description: this._description(),
      duration: Duration.parseString(this._duration()).toProto(),
      displayName: this._displayName(),
      from: new Daytime({
        hour: BigInt(+(hour || 0)),
        minute: BigInt(+(minute || 0)),
      }),
      order: BigInt(this._order()),
      requiredStaffCount: BigInt(this._staffCount()),
      tags: this._tags(),
      timeWorth: this._timeValue() ? Duration.parseString(this._timeValue()).toProto() : undefined,
      eligibleRoleIds: this._eligibleRoles(),
      name: this._name(),
      onHoliday: this._onHoliday(),
    }

    return common;
  })

  protected save() {
    let p: Promise<unknown>;
    if (!this.dialogData || !this.dialogData.id) {
      // we're creating a new one

      p = this.workShiftService
        .createWorkShift(this._computedCurrentModel())

    } else {
      // we're editing an existing one

      p = this.workShiftService
        .updateWorkShift({
          id: this.dialogData.id,
          update: {
            ...this._computedCurrentModel(),
          },
          updateInPlace: this._updateInPlace(),
        })
    }

    p.then(() => toast.success('Schicht wurde erfolgreich gespeichert'))
      .then(() => this.dialogRef.close('change'))
      .catch(err => toast.error(`Schicht konnte nicht gespeichert werden: ${ConnectError.from(err).message}`));
  }

  protected removeTag(item: string) {
    this._tags.set(this._tags().filter(value => value !== item))
  }

  protected removeLastTag() {
    const tags = this._tags();
    if (tags.length === 0) {
      return
    }


    this.removeTag(tags[tags.length - 1]);
  }


  protected addTag(event: KeyboardEvent) {
    this._tags.set([
      ...this._tags(),
      (event.target as HTMLInputElement).value,
    ]);

    (event.target as HTMLInputElement).value = '';
  }

  constructor() {
    const nonIdempotentProperties: (keyof WorkShift)[] = [
      'from', 'duration', 'onHoliday', 'timeWorth', 'days'
    ];

    effect(() => {
      const model = this._computedCurrentModel();
      let hasNonIdempotentChanges = false;

      if (this.dialogData && this.dialogData.id) {
        for (const prop of nonIdempotentProperties) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          if (JSON.stringify(this.dialogData[prop]) !== JSON.stringify((model as any)[prop])) {
            hasNonIdempotentChanges = true;

            break;
          }
        }
      }

      this._updateInPlace.set(!hasNonIdempotentChanges);
    }, {
      allowSignalWrites: true,
    })
  }

  ngOnInit() {
    this.roleService.listRoles({})
      .then(response => this._roles.set(response.roles))
      .then(() => this.assignExisting())
      .catch(err => toast.error(`Failed to load roles: ${ConnectError.from(err).message}`))

  }

  protected abort() {
    this.dialogRef.close();
  }
}
