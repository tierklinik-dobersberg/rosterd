import { DIALOG_DATA } from '@angular/cdk/dialog';
import { BrnDialogModule } from '@spartan-ng/ui-dialog-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { toast } from 'ngx-sonner';

import { ChangeDetectionStrategy, ChangeDetectorRef, Component, DestroyRef, OnInit, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import { ConnectError } from '@connectrpc/connect';
import { BrnDialogRef } from '@spartan-ng/ui-dialog-brain';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { HlmCardModule } from '@tierklinik-dobersberg/angular/card';
import { injectOfftimeService, injectUserService } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule } from '@tierklinik-dobersberg/angular/dialog';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { OffTimeEntry, OffTimeType, Profile } from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzSelectModule } from 'ng-zorro-antd/select';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';

interface CreateModel {
  id?: string;
  requestorId?: string;
  from: Date;
  to: Date;
  comment: string;
  type: 'auto' | 'vacation' | 'timeoff'
}

function makeEmptyCreateModel(): CreateModel {
  return {
    from: new Date(),
    to: new Date(),
    comment: '',
    type: 'auto'
  }
}

@Component({
  selector: 'app-create-offtime',
  standalone: true,
  imports: [
    FormsModule,
    TkdRoster2Module,
    NzRadioModule,
    NzSelectModule,
    RouterModule,
    NzDatePickerModule,
    NzAvatarModule,
    HlmCardModule,
    HlmButtonModule,
    HlmAvatarModule,
    HlmInputModule,
    HlmLabelModule,
    BrnSelectModule,
    HlmSelectModule,
    HlmDialogModule,
    BrnDialogModule,
  ],
  templateUrl: './create-offtime.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class CreateOfftimeComponent implements OnInit {
  private readonly offTimeService = injectOfftimeService();
  private readonly usersService = injectUserService();
  private readonly destroyRef = inject(DestroyRef);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly data = inject(DIALOG_DATA, { optional: true });
  private readonly dialogRef = inject(BrnDialogRef);

  readonly editId = input('');

  model = makeEmptyCreateModel();
  originalModel = makeEmptyCreateModel();

  protected readonly _profiles = signal<Profile[] | null>(null);

  async save() {
    if (this.model.id) {
      const paths: string[] = [];

      if (this.model.comment !== this.originalModel.comment) {
        paths.push("description");
      }

      if (this.model.from.getTime() != this.originalModel.from.getTime()) {
        paths.push("from")
      }

      if (this.model.to.getTime() != this.originalModel.to.getTime()) {
        paths.push("to")
      }

      if (this.model.requestorId != this.originalModel.requestorId) {
        paths.push("requestor_id")
      }

      if (this.model.type != this.originalModel.type) {
        paths.push("request_type")
      }

      await this.offTimeService.updateOffTimeRequest({
        id: this.model.id,
        description: this.model.comment,
        from: Timestamp.fromDate(this.model.from),
        to: Timestamp.fromDate(this.model.to),
        requestorId: this.model.requestorId,
        requestType: this.typeToProto(this.model.type),
        fieldMask: {
          paths,
        }
      })
        .then(() => {
          toast.success(`Antrag wurde erfolgreich bearbeitet!`)
          this.dialogRef.close();
        })
        .catch((err: unknown) => {
          toast.error(ConnectError.from(err).rawMessage)
        })

    } else {
      // Create a new entry
      await this.offTimeService.createOffTimeRequest({
        description: this.model.comment,
        from: Timestamp.fromDate(this.model.from),
        to: Timestamp.fromDate(this.model.to),
        requestType: this.typeToProto(this.model.type),
        requestorId: this.model.requestorId,
      })
        .then(() => {
          toast.success(`Antrag wurde erfolgreich erstellt!`)

          this.dialogRef.close();
        })
        .catch(err => {
          toast.error(ConnectError.from(err).rawMessage);
        })
    }
  }

  private typeToProto(rt: CreateModel['type']): OffTimeType {
    switch (rt) {
      case 'auto':
        return OffTimeType.UNSPECIFIED
      case 'timeoff':
        return OffTimeType.TIME_OFF
      case 'vacation':
        return OffTimeType.VACATION
    }
  }

  ngOnInit(): void {
    this.usersService.listUsers({})
      .then(response => {
        this._profiles.set(response.users);
      })

    if (this.data && 'entry' in this.data && this.data.entry) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      this.applyToModel((this.data as any).entry)
    } else {
      this.model = makeEmptyCreateModel();
    }
  }

  private applyToModel(entry: OffTimeEntry) {
    this.model = {
      id: entry.id,
      comment: entry.description,
      from: entry.from!.toDate(),
      to: entry.to!.toDate(),
      type: 'auto',
      requestorId: entry.requestorId
    };

    switch (entry.type) {
      case OffTimeType.UNSPECIFIED:
        this.model.type = 'auto';
        break;
      case OffTimeType.TIME_OFF:
        this.model.type = 'timeoff';
        break;
      case OffTimeType.VACATION:
        this.model.type = 'vacation';
        break;
    }

    // clone the model type so we can compare for changes
    // when updating.
    this.originalModel = {
      ...this.model
    };
  }

  abort() {
    this.dialogRef.close();
  }
}
