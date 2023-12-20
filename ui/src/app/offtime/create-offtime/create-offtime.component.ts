import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import { OffTimeType, Profile } from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzSelectModule } from 'ng-zorro-antd/select';
import { OFFTIME_SERVICE, USER_SERVICE } from '@tierklinik-dobersberg/angular/connect';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';
import { NzMessageModule, NzMessageService } from 'ng-zorro-antd/message';
import { ConnectError } from '@connectrpc/connect';

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
    CommonModule,
    FormsModule,
    TkdRoster2Module,
    NzRadioModule,
    NzSelectModule,
    RouterModule,
    NzDatePickerModule,
    NzAvatarModule,
    NzMessageModule
  ],
  templateUrl: './create-offtime.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class CreateOfftimeComponent implements OnInit {
  private readonly offTimeService = inject(OFFTIME_SERVICE);
  private readonly usersService = inject(USER_SERVICE);
  private readonly route = inject(ActivatedRoute)
  private readonly router = inject(Router)
  private readonly destroyRef = inject(DestroyRef);
  private readonly nzMessage = inject(NzMessageService)
  private readonly cdr = inject(ChangeDetectorRef);

  model = makeEmptyCreateModel();
  originalModel = makeEmptyCreateModel();

  profiles: Profile[] = [];

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
        this.router.navigate(['/offtimes'])
      })
      .catch(err => {
        this.nzMessage.error(ConnectError.from(err).rawMessage)
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
        this.router.navigate(['/offtimes'])
      })
      .catch(err => {
        this.nzMessage.error(ConnectError.from(err).rawMessage);
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
        this.profiles = response.users;
        this.cdr.markForCheck();
      })

    this.route
      .data
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(data => {
        if (!!data['isNewEntry']) {
          this.model = makeEmptyCreateModel();
        } else {
          this.offTimeService.getOffTimeEntry({
            ids: [this.route.snapshot.paramMap.get("id")!],
          }).then(response => {
            this.model = {
              id: response.entry[0].id,
              comment: response.entry[0].description,
              from: response.entry[0].from!.toDate(),
              to: response.entry[0].to!.toDate(),
              type: 'auto',
              requestorId: response.entry[0].requestorId
            };

            switch (response.entry[0].type) {
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

            this.cdr.markForCheck();
          })
        }
      })
  }
}
