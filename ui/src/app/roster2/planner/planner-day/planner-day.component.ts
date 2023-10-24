import { CdkOverlayOrigin } from "@angular/cdk/overlay";
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, OnChanges, Output, SimpleChanges } from "@angular/core";
import { PartialMessage } from "@bufbuild/protobuf";
import { OffTimeEntry, PlannedShift, Profile, PublicHoliday, WorkTimeAnalysis } from "@tkd/apis";
import { NzModalService } from "ng-zorro-antd/modal";
import { RosterShift } from "../roster-planner.component";

@Component({
  selector: 'tkd-roster-planner-day',
  templateUrl: './planner-day.html',
  changeDetection:ChangeDetectionStrategy.OnPush,
  styles: [
    `
    :host {
      display: flex;
      flex-direction: column;
      height: 200px;
      overflow: hidden;
    }
    `
  ],
})
export class TkdRosterPlannerDayComponent implements OnChanges {
  @Input()
  date!: Date;

  @Input()
  readonly: boolean = false;

  @Input()
  rosterDate!: Date;

  @Input()
  requiredShifts: RosterShift[] = [];

  @Input()
  profiles: Profile[] = [];

  @Input()
  selectedUser: string | null = null;

  @Input()
  holiday?: PublicHoliday;

  @Input()
  highlightUserShifts: string | null = null;

  @Input()
  offTimeRequest: {[id: string]: OffTimeEntry[] | undefined } = {}

  @Input()
  workTimeStatus: {[user: string]: WorkTimeAnalysis | undefined } = {}

  assigned: {
    [id: string]: Set<string> | undefined
  } = {};

  @Input()
  plannedShifts: PlannedShift[] = [];

  @Output()
  rosterShiftChange = new EventEmitter<PartialMessage<PlannedShift>[]>();

  get isDisabledDate() {
    return this.date.getMonth() !== this.rosterDate.getMonth();
  }

  constructor(
    private nzModal: NzModalService,
    private cdr: ChangeDetectorRef
  ) {}

  onShiftClick(trigger: CdkOverlayOrigin, shift: RosterShift, user = this.selectedUser) {
    if (this.readonly) {
      return;
    }

    if (!!user) {
      let set = this.assigned[shift.workShiftId!] || new Set();
      let assign = () => {
        this.assigned[shift.workShiftId!] = set

        if (set.has(user)) {
          set.delete(user);
        } else {
          set.add(user)
        }

        // for CD, we need to make new instances of all sets
        // so the inList pipe will get fired again.
        let newAssigned: {
          [id: string]: Set<string>
        } = {};

        Object.keys(this.assigned).forEach(key => {
          newAssigned[key] = new Set(this.assigned[key]?.values())
        })
        this.assigned = newAssigned;

        this.cdr.markForCheck();
        this.publishRosterShift();
      }

      let confirmMessage: any = '';
      if (!set.has(user)) {
        if (!shift.eligibleUserIds?.includes(user)) {
          const profile = this.profiles.find(p => p.user?.id === user);
          const displayName = profile?.user?.displayName || profile?.user?.username || 'N/A';

          confirmMessage =  `Benutzer ${displayName} ist für die ausgewählte Schicht nicht berechtigt.`;

          if (!!shift.violationsPerUserId) {
            if (!!shift.violationsPerUserId[user]?.violations?.length) {
              let reason = '';
              shift.violationsPerUserId[user].violations!.forEach(vio => {
                switch (vio.kind?.case) {
                  case 'evaluation':
                    reason += 'Evaluation: '  + vio.kind.value.description
                    break;
                  case 'offTime':
                    reason += 'OffTime: ' + vio.kind.value.entry?.description
                }

              })

              confirmMessage = `Benutzer ${displayName} ist aus folgenden Gründen für diese Schicht gesperrt:` + reason;
            }
          }

        } else if (shift.definition.requiredStaffCount > 0 && (this.assigned[shift.workShiftId!]?.size || 0) >= shift.definition.requiredStaffCount) {
          confirmMessage = `Es sind bereits genügend Mitarbeiter dieser Schicht zugewiesen.`
        }
      }

      if (!!confirmMessage) {
        this.nzModal.confirm({
          nzTitle: 'Bestätigung erforderlich',
          nzContent: confirmMessage + ' Möchtest du trotzdem fortfahren?',
          nzOkText: 'Zuweisen',
          nzOnOk: assign,
        })
      } else {
        assign()
      }

      return
    }

    trigger.elementRef.nativeElement.open = !trigger.elementRef.nativeElement.open;
  }

  onOverlayOutsideClick(event: MouseEvent, trigger: CdkOverlayOrigin) {
    let iter: HTMLElement | null = event.target as HTMLElement;
    while (!!iter) {
      if (iter === trigger.elementRef.nativeElement) {
        return;
      }

      iter = iter.parentElement;
    }

    trigger.elementRef.nativeElement.open = false;
  }

  onContextMenu(event: Event, trigger: CdkOverlayOrigin) {
    if (this.readonly) {
      return;
    }

    // stop immediate propergation as onShiftClick() would close the
    // overlay immediately again.
    event.stopImmediatePropagation();
    event.preventDefault();

    trigger.elementRef.nativeElement.open = !trigger.elementRef.nativeElement.open;
    return false;
  }

  private publishRosterShift() {
    if (this.readonly) {
      return;
    }

    this.rosterShiftChange.next(
      Object.keys(this.assigned)
        .map(shiftID => {
          const required = this.requiredShifts.find(s => s.workShiftId === shiftID);

          if (!required) {
            return {}
          }

          return <PartialMessage<PlannedShift>>{
            assignedUserIds: Array.from(this.assigned[shiftID] || []),
            from: required!.from,
            to: required!.to,
            workShiftId: shiftID,
          }
        })
    )
  }

  ngOnChanges(changes: SimpleChanges) {
    if ('requiredShifts' in changes || 'plannedShifts' in changes) {
      this.assigned = {};

      this.plannedShifts?.forEach(shift => {
        this.assigned[shift.workShiftId] = new Set(shift.assignedUserIds)
      })
    }
  }
}