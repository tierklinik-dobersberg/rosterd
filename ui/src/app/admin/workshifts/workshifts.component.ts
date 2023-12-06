import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { PartialMessage, Duration as ProtoDuration } from '@bufbuild/protobuf';
import { Daytime, Role, WorkShift } from '@tierklinik-dobersberg/apis';
import { NzCheckboxModule } from 'ng-zorro-antd/checkbox';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzSelectModule } from 'ng-zorro-antd/select';
import { ROLE_SERVICE, WORK_SHIFT_SERVICE } from '@tierklinik-dobersberg/angular/connect';
import { DaytimePipe, DurationPipe } from '@tierklinik-dobersberg/angular/pipes';
import { TkdRoster2Module } from 'src/app/roster2/roster2.module';
import { Duration } from 'src/duration';
import { padLeft } from 'src/utils';

interface LocalWorkShift {
  id: string;
  color: string;
  description: string;
  displayName: string;
  duration: ProtoDuration;
  timeWorth: ProtoDuration;
  tags: string[];
  name: string;
  order: number;
  requiredStaffCount: number;
  from: string;
  eligibleRoleIds: string[];
  days: number[];
  onHoliday: boolean;
}

function makeEmptyWorkShift(): LocalWorkShift {
  return {
    id: '',
    days: [],
    color: '',
    description: '',
    duration: new ProtoDuration({ seconds: BigInt(0) }),
    displayName: '',
    from: '',
    order: 0,
    requiredStaffCount: 0,
    tags: [],
    timeWorth: new ProtoDuration({ seconds: BigInt(0) }),
    eligibleRoleIds: [],
    name: '',
    onHoliday: false,
  };
}

@Component({
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    TkdRoster2Module,
    NzRadioModule,
    NzSelectModule,
    NzCheckboxModule,
    DaytimePipe,
    DurationPipe,
  ],
  templateUrl: './workshifts.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class WorkshiftsComponent implements OnInit {
  readonly workShiftService = inject(WORK_SHIFT_SERVICE);
  readonly roleService = inject(ROLE_SERVICE);
  readonly route = inject(ActivatedRoute);
  readonly destroyRef = inject(DestroyRef);
  readonly cdr = inject(ChangeDetectorRef);
  readonly router = inject(Router);

  readonly parseDuration = Duration.parseString;

  workShift: LocalWorkShift = makeEmptyWorkShift()
  originalWorkShift: LocalWorkShift | null = null;

  roles: Role[] = [];

  updateInPlace: boolean = false;

  async delete() {
    await this.workShiftService.deleteWorkShift({
      id: this.workShift.id,
    })

    this.router.navigate(['/admin'])
  }

  async save() {
    const parts = this.workShift.from.split(':')

    let hour = BigInt(+parts[0])
    let minute = BigInt(+parts[1])

    if (!!this.workShift.id) {
      const response = await this.workShiftService.updateWorkShift({
        id: this.workShift.id,
        update: {
          ...this.workShift,
          from: new Daytime({
            hour: hour,
            minute: minute,
          }),
          requiredStaffCount: BigInt(this.workShift.requiredStaffCount),
          order: BigInt(this.workShift.order),
        },
        updateInPlace: this.updateInPlace,
      })

      this.workShift.id = response.workShift!.id;
    } else {

      const response = await this.workShiftService.createWorkShift({
          ...this.workShift,
          from: new Daytime({
            hour: hour,
            minute: minute,
          }),
          requiredStaffCount: BigInt(this.workShift.requiredStaffCount),
          order: BigInt(this.workShift.order),
      })

      this.workShift.id = response.workShift!.id;
    }

    this.router.navigate(['/admin'])
  }

  handleChanges() {
    const nonIdempotentProperties: (keyof LocalWorkShift)[] = [
      'from', 'duration', 'onHoliday', 'timeWorth', 'days'
    ];

    let hasNonIdempotentChanges = false;

    if (!!this.originalWorkShift) {
      for (let prop of nonIdempotentProperties) {
        if (JSON.stringify(this.originalWorkShift[prop]) !== JSON.stringify(this.workShift[prop])) {
          hasNonIdempotentChanges = true;

          break;
        }
      }
    }

    this.updateInPlace = !hasNonIdempotentChanges;
  }

  async ngOnInit() {
    this.roleService.listRoles({})
      .then(response => {
        this.roles = response.roles;
        this.cdr.markForCheck();
      });

    this.route.data
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(data => {
        if (!!data['isNewEntry']) {
          this.workShift = makeEmptyWorkShift();
          this.originalWorkShift = null;
        } else {

          this.workShiftService.listWorkShifts({})
            .then(response => {
              const ws: PartialMessage<WorkShift> = response.workShifts
                .find(ws => ws.id === this.route.snapshot.paramMap.get('id')) || {}

              this.workShift = {
                color: ws.color || '',
                days: ws.days || [],
                description: ws.description || '',
                displayName: ws.displayName || '',
                duration: new ProtoDuration(ws.duration),
                eligibleRoleIds: ws.eligibleRoleIds || [],
                from: ws.from ? (padLeft(`${ws.from!.hour}`, 2, '0') + ':' + padLeft(`${ws.from!.minute}`, 2, '0')) : '00:00',
                id: ws.id || '',
                name: ws.name || '',
                onHoliday: ws.onHoliday || false,
                order: Number(ws.order || 0),
                requiredStaffCount: Number(ws.requiredStaffCount || 0),
                tags: ws.tags || [],
                timeWorth: new ProtoDuration(ws.timeWorth)
              }

              this.originalWorkShift = {...this.workShift};

              this.cdr.markForCheck();
            });
        }
      })
  }
}
