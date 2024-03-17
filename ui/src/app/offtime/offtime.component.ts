import { CommonModule, DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  OnInit,
  TrackByFunction,
  inject,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import {
  ApprovalRequestType,
  OffTimeEntry,
  OffTimeType,
  Profile,
} from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { NzDropDownModule } from 'ng-zorro-antd/dropdown';
import { NzModalModule, NzModalService } from 'ng-zorro-antd/modal';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzToolTipModule } from 'ng-zorro-antd/tooltip';
import { NzTableModule, NzTableSortFn } from 'ng-zorro-antd/table';
import {
  OFFTIME_SERVICE,
  USER_SERVICE,
} from '@tierklinik-dobersberg/angular/connect';
import { TkdRoster2Module } from '../roster2/roster2.module';
import {
  DisplayNamePipe,
  ToUserPipe,
} from '@tierklinik-dobersberg/angular/pipes';
import { NgIconsModule } from '@ng-icons/core';
import { NzSelectModule } from 'ng-zorro-antd/select';

@Component({
  selector: 'app-offtime',
  standalone: true,
  imports: [
    CommonModule,
    TkdRoster2Module,
    NzAvatarModule,
    NzModalModule,
    FormsModule,
    RouterModule,
    NzDropDownModule,
    NzToolTipModule,
    NzDatePickerModule,
    NzRadioModule,
    NgIconsModule,
    NzSelectModule,
    NzTableModule,
  ],
  templateUrl: './offtime.component.html',
  styles: [
    `
      :host {
        @apply flex flex-col overflow-hidden flex-grow;
      }
    `,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OfftimeComponent implements OnInit {
  private readonly offTimeService = inject(OFFTIME_SERVICE);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly userService = inject(USER_SERVICE);
  private readonly modalService = inject(NzModalService);
  readonly types = OffTimeType;

  rangeFilter: [Date, Date] | null = null;
  filterType: 'all' | 'new' = 'all';
  filterByUser: string[] = [];

  profiles: Profile[] = [];
  entries: OffTimeEntry[] = [];
  approvalComment = '';
  approvalModalEntry: OffTimeEntry | null = null;
  approvalModalApprove: 'approve' | 'reject' = 'approve';

  // sort functions
  sortByUser: NzTableSortFn<OffTimeEntry> = (a, b) => {
    const userA = this.profiles.find(
      (profile) => profile.user!.id === a.requestorId
    );
    const userB = this.profiles.find(
      (profile) => profile.user!.id === b.requestorId
    );

    if (!userA) {
      return -1;
    }

    if (!userB) {
      return -1;
    }

    return userA.user!.username.localeCompare(userB.user!.username);
  };

  sortByStartDate: NzTableSortFn<OffTimeEntry> = (a, b) =>
    a.from!.toDate().getSeconds() - b.from!.toDate().getSeconds();

  sortByEndDate: NzTableSortFn<OffTimeEntry> = (a, b) =>
    a.to!.toDate().getSeconds() - b.to!.toDate().getSeconds();

  sortByCreated: NzTableSortFn<OffTimeEntry> = (a, b) =>
    a.createdAt!.toDate().getSeconds() - b.createdAt!.toDate().getSeconds();

  sortByApproval: NzTableSortFn<OffTimeEntry> = (a, b) => {
    if (!a.approval) {
      return -1;
    }

    if (!b.approval) {
      return -1;
    }

    return 0;
  };

  sortByType: NzTableSortFn<OffTimeEntry> = (a, b) =>
    a.type.valueOf() - b.type.valueOf();

  trackEntry: TrackByFunction<OffTimeEntry> = (_, e) => e.id;

  ngOnInit() {
    this.userService.listUsers({}).then((response) => {
      this.profiles = response.users;
      this.cdr.markForCheck();
    });

    this.loadOffTimeEntries();
  }

  async loadOffTimeEntries() {
    if (this.rangeFilter) {
      if (this.rangeFilter[0]) {
        this.rangeFilter[0].setUTCHours(0);
        this.rangeFilter[0].setUTCMinutes(0);
        this.rangeFilter[0].setUTCSeconds(0);
      }
      if (this.rangeFilter[1]) {
        this.rangeFilter[1].setUTCHours(23);
        this.rangeFilter[1].setUTCMinutes(59);
        this.rangeFilter[1].setUTCSeconds(59);
      }
    }

    this.offTimeService
      .findOffTimeRequests({
        from:
          this.rangeFilter && this.rangeFilter[0]
            ? Timestamp.fromDate(this.rangeFilter[0])
            : undefined,
        to:
          this.rangeFilter && this.rangeFilter[1]
            ? Timestamp.fromDate(this.rangeFilter[1])
            : undefined,
      })
      .then((response) => {
        this.entries = response.results.filter((value) => {
          if (this.filterType !== 'all') {
            if (value.approval) {
              return false;
            }
          }

          if (!this.filterByUser || this.filterByUser.length === 0) {
            return true;
          }

          return this.filterByUser.includes(value.requestorId);
        });
        this.cdr.markForCheck();
      });
  }

  approveOrRejectConfirmation(approve: boolean, entry: OffTimeEntry) {
    this.approvalModalApprove = approve ? 'approve' : 'reject';
    this.approvalComment = '';
    this.approvalModalEntry = entry;

    this.cdr.markForCheck();
  }

  async approveOrReject() {
    if (!this.approvalModalEntry) {
      return;
    }

    await this.offTimeService.approveOrReject({
      id: this.approvalModalEntry.id,
      comment: this.approvalComment,
      type:
        this.approvalModalApprove === 'approve'
          ? ApprovalRequestType.APPROVED
          : ApprovalRequestType.REJECTED,
    });

    this.approvalModalEntry = null;
    await this.loadOffTimeEntries();
  }

  async deleteEntry(entry: OffTimeEntry) {
    const username = new DisplayNamePipe().transform(
      new ToUserPipe().transform(entry.requestorId, this.profiles)
    );
    const from = new DatePipe('de-AT').transform(entry.from!.toDate(), 'short');
    const to = new DatePipe('de-AT').transform(entry.to!.toDate(), 'short');

    this.modalService.confirm({
      nzTitle: 'Bestätigung erforderlich',
      nzContent: `Bist du sicher dass du den Urlaubsantrag "${entry.description}" von ${username} vom ${from} bis ${to} löschen möchtest?`,
      nzOkDanger: true,
      nzOkText: 'Ja, löschen',
      nzCancelText: 'Abbrechen',
      nzOnOk: () => {
        this.offTimeService
          .deleteOffTimeRequest({ id: [entry.id!] })
          .then(() => this.loadOffTimeEntries());
      },
    });
  }
}
