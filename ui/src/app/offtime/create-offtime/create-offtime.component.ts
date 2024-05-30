import { DIALOG_DATA } from '@angular/cdk/dialog';
import { BrnDialogModule } from '@spartan-ng/ui-dialog-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { toast } from 'ngx-sonner';

import { ChangeDetectionStrategy, Component, OnInit, computed, inject, input, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import { ConnectError } from '@connectrpc/connect';
import { BrnDialogRef } from '@spartan-ng/ui-dialog-brain';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmBadgeModule } from '@tierklinik-dobersberg/angular/badge';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { HlmCardModule } from '@tierklinik-dobersberg/angular/card';
import { HlmCheckboxModule } from '@tierklinik-dobersberg/angular/checkbox';
import { injectOfftimeService, injectUserService } from '@tierklinik-dobersberg/angular/connect';
import { HlmDialogModule } from '@tierklinik-dobersberg/angular/dialog';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { OffTimeEntry, OffTimeType, Profile } from '@tierklinik-dobersberg/apis';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';


@Component({
  selector: 'app-create-offtime',
  standalone: true,
  imports: [
    FormsModule,
    TkdRoster2Module,
    RouterModule,
    NzDatePickerModule,
    HlmCardModule,
    HlmButtonModule,
    HlmAvatarModule,
    HlmInputModule,
    HlmLabelModule,
    HlmCheckboxModule,
    BrnSelectModule,
    HlmSelectModule,
    HlmDialogModule,
    BrnDialogModule,
    HlmBadgeModule
  ],
  templateUrl: './create-offtime.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class CreateOfftimeComponent implements OnInit {
  private readonly offTimeService = injectOfftimeService();
  private readonly usersService = injectUserService();
  private readonly data: { entry: OffTimeEntry } = inject(DIALOG_DATA, { optional: true });
  private readonly dialogRef = inject(BrnDialogRef);

  readonly editId = input('');

  protected readonly _range = model<[Date, Date]>();
  protected readonly _requestor = model<string>();
  protected readonly _comment = model<string>();
  protected readonly _type = model<OffTimeType>();
  protected readonly Types = OffTimeType;

  protected readonly _isEdit = signal(false);

  protected readonly _computedCurrentModel = computed(() => {
    const description = this._comment();
    const range = this._range();
    const requestor = this._requestor();
    const type = this._type();

    if (!range) {
      return null;
    }

    if (!requestor) {
      return null;
    }

    const result = {
      description: description || '',
      from: Timestamp.fromDate(range[0]),
      to: Timestamp.fromDate(range[1]),
      requestorId: requestor,
      requestType: type || OffTimeType.UNSPECIFIED,
    }

    return result
  })

  protected readonly _profiles = signal<Profile[] | null>(null);

  protected _wholeDays = model(true);

  async save() {
    const current = this._computedCurrentModel();

    if (!current) {
      return;
    }

    if (this.data && this.data.entry) {
      const paths: string[] = [];
      const original = this.data.entry;

      if (current.description !== original.description) {
        paths.push("description");
      }

      if (current.from.toDate().getTime() != original.from?.toDate().getTime()) {
        paths.push("from")
      }

      if (current.to.toDate().getTime() != original.to?.toDate().getTime()) {
        paths.push("to")
      }

      if (current.requestorId != original.requestorId) {
        paths.push("requestor_id")
      }

      if (current.requestType != original.type) {
        paths.push("request_type")
      }

      await this.offTimeService.updateOffTimeRequest({
        id: this.data.entry.id,
        ...current,
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
        ...current,
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

  ngOnInit(): void {
    this.usersService.listUsers({})
      .then(response => {
        this._profiles.set(response.users);
      })

    if (this.data && 'entry' in this.data && this.data.entry) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      this.applyToModel((this.data as any).entry)
    }
  }

  private applyToModel(entry: OffTimeEntry) {
    this._isEdit.set(true);

    this._comment.set(entry.description);
    this._range.set([entry.from!.toDate(), entry.to!.toDate()])
    this._requestor.set(entry.requestorId)
    this._type.set(entry.type)
  }

  abort() {
    this.dialogRef.close();
  }
}
