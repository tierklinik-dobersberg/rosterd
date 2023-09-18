import { CommonModule, DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, TrackByFunction, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { Timestamp } from '@bufbuild/protobuf';
import { ApprovalRequestType, OffTimeEntry, Profile } from '@tkd/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDatePickerModule } from 'ng-zorro-antd/date-picker';
import { NzDropDownModule } from 'ng-zorro-antd/dropdown';
import { NzModalModule, NzModalService } from 'ng-zorro-antd/modal';
import { NzRadioModule } from 'ng-zorro-antd/radio';
import { NzToolTipModule } from 'ng-zorro-antd/tooltip';
import { OFFTIME_SERVICE, USER_SERVICE } from '../connect_clients';
import { TkdRoster2Module } from '../roster2/roster2.module';
import { DisplayNamePipe, ToUserPipe } from '../roster2/to-user.pipe';

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
    NzRadioModule
  ],
  templateUrl: './offtime.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class OfftimeComponent implements OnInit {
  private readonly offTimeService = inject(OFFTIME_SERVICE);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly userService = inject(USER_SERVICE);
  private readonly modalService = inject(NzModalService);

  rangeFilter: [Date, Date] | null = null;
  filterType: 'all' | 'new' = 'all';

  profiles: Profile[] = [];
  entries: OffTimeEntry[] = [];
  approvalComment = '';
  approvalModalEntry: OffTimeEntry | null = null;
  approvalModalApprove: 'approve' | 'reject' = 'approve';

  trackEntry: TrackByFunction<OffTimeEntry> = (_, e) => e.id;

  ngOnInit() {
    this.userService.listUsers({})
      .then(response => {
        this.profiles = response.users;
        this.cdr.markForCheck();
      })

    this.loadOffTimeEntries()
  }

  async loadOffTimeEntries() {
    if (this.rangeFilter) {
      if (this.rangeFilter[0]) {
        this.rangeFilter[0].setUTCHours(0)
        this.rangeFilter[0].setUTCMinutes(0)
        this.rangeFilter[0].setUTCSeconds(0)
      }
      if (this.rangeFilter[1]) {
        this.rangeFilter[1].setUTCHours(23)
        this.rangeFilter[1].setUTCMinutes(59)
        this.rangeFilter[1].setUTCSeconds(59)
      }
    }

    this.offTimeService.findOffTimeRequests({
      from: this.rangeFilter && this.rangeFilter[0] ? Timestamp.fromDate(this.rangeFilter[0]) : undefined,
      to: this.rangeFilter && this.rangeFilter[1] ? Timestamp.fromDate(this.rangeFilter[1]) : undefined,
    })
      .then(response => {
        this.entries = response.results;
        this.cdr.markForCheck();
      })
  }

  approveOrRejectConfirmation(approve: boolean, entry: OffTimeEntry) {
    this.approvalModalApprove = approve ? 'approve' : 'reject';
    this.approvalComment = '';
    this.approvalModalEntry = entry;

    this.cdr.markForCheck();
  }

  async approveOrReject() {
    if (!this.approvalModalEntry) {
      return
    }

    await this.offTimeService.approveOrReject({
      id: this.approvalModalEntry.id,
      comment: this.approvalComment,
      type: this.approvalModalApprove === 'approve' ? ApprovalRequestType.APPROVED : ApprovalRequestType.REJECTED,
    })

    this.approvalModalEntry = null;
    await this.loadOffTimeEntries();
  }

  async deleteEntry(entry: OffTimeEntry) {
    const username = new DisplayNamePipe().transform(new ToUserPipe().transform(entry.requestorId, this.profiles));
    const from = new DatePipe("de-AT").transform(entry.from!.toDate(), 'short');
    const to = new DatePipe("de-AT").transform(entry.to!.toDate(), 'short');

    this.modalService.confirm({
        nzTitle: "Bestätigung erforderlich" ,
        nzContent: `Bist du sicher dass du den Urlaubsantrag "${entry.description}" von ${username} vom ${from} bis ${to} löschen möchtest?`,
        nzOkDanger: true,
        nzOkText: "Ja, löschen",
        nzCancelText: "Abbrechen",
        nzOnOk: () => {
          this.offTimeService.deleteOffTimeRequest({ id: [entry.id!] })
            .then(() => this.loadOffTimeEntries())
        }
    })
  }
}
