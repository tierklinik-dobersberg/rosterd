import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, Pipe, PipeTransform, TrackByFunction, inject } from '@angular/core';
import { RouterModule } from '@angular/router';
import { Constraint, Daytime, Role, WorkShift } from '@tkd/apis';
import { padLeft } from 'src/utils';
import { CONSTRAINT_SERVICE, ROLES_SERVICE, WORKSHIFT_SERVICE } from '../connect_clients';
import { TkdRoster2Module } from '../roster2/roster2.module';

@Pipe({
  name: 'daytime',
  pure: true,
  standalone: true,
})
export class DaytimePipe implements PipeTransform {
  transform(value?: Daytime, ...args: any[]) {
    if (!value) {
      return ''
    }

    return padLeft(`${value.hour}`, 2, '0') + ':' + padLeft(`${value.minute}`, 2, '0')
  }
}

enum Workday {
  So,
  Mo,
  Di,
  Mi,
  Do,
  Fr,
  Sa,
}

@Pipe({
  name: 'workday',
  pure: true,
  standalone: true,
})
export class WorkDayPipe implements PipeTransform {

  transform(value?: number[], ...args: any[]) {
    if (!value) {
      return ''
    }

    return value
      .map(day => Workday[day])
      .join(', ')
  }
}

@Pipe({
  name: 'roleList',
  pure: true,
  standalone: true,
})
export class RoleListPipe implements PipeTransform {

  transform(value: string[] | undefined, roles: Role[]) {
    if (!value) {
      return ''
    }

    return value
      .map(id => roles.find(role => role.id === id))
      .filter(role => !!role)
      .map(role => role!.name)
      .join(', ')
  }
}

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
  private readonly workShiftService = inject(WORKSHIFT_SERVICE);
  private readonly constraintService = inject(CONSTRAINT_SERVICE);
  private readonly roleService = inject(ROLES_SERVICE);

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
