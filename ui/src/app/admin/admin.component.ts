import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, TrackByFunction, inject } from '@angular/core';
import { RouterModule } from '@angular/router';
import { Constraint, Role, WorkShift } from '@tkd/apis';
import { DaytimePipe, RoleListPipe, WorkDayPipe } from '@tkd/angular/pipes';
import { TkdRoster2Module } from '../roster2/roster2.module';
import { WORK_SHIFT_SERVICE, CONSTRAINT_SERVICE, ROLE_SERVICE } from '@tkd/angular/connect';


@Component({
  standalone: true,
  imports: [
    CommonModule,
    RouterModule,
    TkdRoster2Module,
    DaytimePipe,
    WorkDayPipe,
    RoleListPipe,
  ],
  templateUrl: './admin.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminComponent implements OnInit {
  private readonly cdr = inject(ChangeDetectorRef)
  private readonly workShiftService = inject(WORK_SHIFT_SERVICE);
  private readonly constraintService = inject(CONSTRAINT_SERVICE);
  private readonly roleService = inject(ROLE_SERVICE);

  workShifts: WorkShift[] = [];
  constraints: Constraint[] = [];
  roles: Role[] = [];

  trackWs: TrackByFunction<WorkShift> = (_, w) => w.id;
  trackCs: TrackByFunction<Constraint> = (_, c) => c.id;

  async ngOnInit() {
    this.roles = await this.roleService
      .listRoles({})
      .then(response => response.roles)

    this.workShifts = await this.workShiftService
      .listWorkShifts({})
      .then(response => response.workShifts)
      .then(shifts => shifts.sort((a, b) => {
        const diff = Number(a.order - b.order);
        if (diff !== 0) {
          return diff;
        }

        if (a.name > b.name) {
          return 1
        }
        if (a.name < b.name) {
          return -1
        }

        return 0
      }))

    this.constraints = await this.constraintService
      .findConstraints({})
      .then(response => response.results)

    this.cdr.markForCheck();
  }
}
