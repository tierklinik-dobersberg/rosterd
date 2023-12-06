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
    NzAvatarModule
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
  private readonly cdr = inject(ChangeDetectorRef);

  model = makeEmptyCreateModel();
  profiles: Profile[] = [];

  async save() {
    if (this.model.id) {
      throw new Error("not yet supported")
    } else {
      await this.offTimeService.createOffTimeRequest({
        description: this.model.comment,
        from: Timestamp.fromDate(this.model.from),
        to: Timestamp.fromDate(this.model.to),
        requestType: (() => {
          switch (this.model.type) {
            case 'auto':
              return OffTimeType.UNSPECIFIED
            case 'timeoff':
              return OffTimeType.TIME_OFF
            case 'vacation':
              return OffTimeType.VACATION
          }
        })()
      })

      this.router.navigate(['/offtimes'])
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
            }

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

            this.cdr.markForCheck();
          })
        }
      })
  }
}
