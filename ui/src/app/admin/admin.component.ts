import { NzDropDownModule } from 'ng-zorro-antd/dropdown';
import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, TrackByFunction, inject } from '@angular/core';
import { RouterModule } from '@angular/router';
import { Constraint, CreateWorkShiftRequest, Role, WorkShift } from '@tkd/apis';
import { DaytimePipe, RoleListPipe, WorkDayPipe } from '@tkd/angular/pipes';
import { TkdRoster2Module } from '../roster2/roster2.module';
import { WORK_SHIFT_SERVICE, CONSTRAINT_SERVICE, ROLE_SERVICE } from '@tkd/angular/connect';
import { NzMessageService } from 'ng-zorro-antd/message';
import { ConnectError } from '@bufbuild/connect';
import { NzModalModule, NzModalService } from 'ng-zorro-antd/modal';


@Component({
  standalone: true,
  imports: [
    CommonModule,
    RouterModule,
    TkdRoster2Module,
    DaytimePipe,
    WorkDayPipe,
    RoleListPipe,
    NzDropDownModule,
    NzModalModule
  ],
  templateUrl: './admin.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminComponent implements OnInit {
  private readonly cdr = inject(ChangeDetectorRef)
  private readonly workShiftService = inject(WORK_SHIFT_SERVICE);
  private readonly constraintService = inject(CONSTRAINT_SERVICE);
  private readonly roleService = inject(ROLE_SERVICE);
  private readonly nzMessageService = inject(NzMessageService);
  private readonly nzModalService = inject(NzModalService)

  workShifts: WorkShift[] = [];
  constraints: Constraint[] = [];
  roles: Role[] = [];

  trackWs: TrackByFunction<WorkShift> = (_, w) => w.id;
  trackCs: TrackByFunction<Constraint> = (_, c) => c.id;

  async ngOnInit() {
    await this.load()
  }

  private async load() {
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

  delete(entry: WorkShift) {
    this.nzModalService
      .confirm({
        nzTitle: 'Schicht löschen',
        nzContent: 'Möchtest du diese Arbeitsschicht wirklich löschen?',
        nzOkDanger: true,
        nzOkText: 'Löschen',
        nzCancelText: 'Nein',
        nzOnOk: async () => {
          await this.workShiftService
            .deleteWorkShift({id: entry.id})
            .catch(err => {
              this.nzMessageService.error('Failed to delete workshift: ' + ConnectError.from(err).rawMessage)
            })

          await this.load();
        }
      })
  }

  async duplicate(entry: WorkShift) {
    const copy = new CreateWorkShiftRequest(entry)
    copy.name += " (Copy)"
    copy.order++;

    const msgRef = this.nzMessageService.loading("Eintrag wird dupliziert")

    await this.workShiftService
      .createWorkShift({
        ...copy,
      })
      .catch(err => {
        this.nzMessageService.error('Failed to copy workshift: ' + ConnectError.from(err).rawMessage)
      })

    await this.load();

    this.nzMessageService.remove(msgRef.messageId)
  }
}
