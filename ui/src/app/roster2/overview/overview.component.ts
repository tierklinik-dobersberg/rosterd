import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, TrackByFunction, inject } from '@angular/core';
import { Profile, Roster, RosterType } from '@tkd/apis';
import { NzMessageService } from 'ng-zorro-antd/message';
import { ROSTER_SERVICE, USER_SERVICE } from 'src/app/connect_clients';

interface RosterWithLink {
  roster: Roster;
  link: string;
}

@Component({
  selector: 'app-overview',
  templateUrl: './overview.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class TkdRosterOverviewComponent implements OnInit {
  rosterService = inject(ROSTER_SERVICE);
  userService = inject(USER_SERVICE);
  messageService = inject(NzMessageService)

  cdr = inject(ChangeDetectorRef);

  rosters: RosterWithLink[] = [];
  profiles: Profile[] = [];
  rosterTypes: RosterType[] = [];

  trackRoster: TrackByFunction<RosterWithLink> = (_, r) => r.roster.id

  nextMonth = (() => {
    const now = new Date();
    const next = new Date(now.getFullYear(), now.getMonth()+1, 1)

    return [next.getFullYear(), next.getMonth()+1]
  })()

  async deleteRoster(roster: Roster) {
    await this.rosterService.deleteRoster({
      id: roster.id,
    })

    await this.loadRosters();

    this.cdr.markForCheck();
  }

  async sendPreview(roster: Roster) {
    await this.rosterService.sendRosterPreview({
      id: roster.id,
    })
    .then(() => {
      this.messageService.success("Alle mails wurden erfolgreich versandt.")
    })
  }

  async loadRosters() {
    this.rosters = await this.rosterService
      .getRoster({
        readMask: {
          paths: ['roster.id', 'roster.from', 'roster.to', 'roster.approved', 'roster.roster_type_name', 'roster.approved_at', 'roster.approver_user_id', 'roster.last_modified_by', 'roster.updated_at'],
        }
      })
      .then(response => response.roster)
      .then(rosters => rosters.map(roster => {
        const date = new Date(roster.from);

        return {
          roster: roster,
          link: `plan/${roster.rosterTypeName}/${date.getFullYear()}/${date.getMonth() + 1}`,
        }
      }))
  }

  async ngOnInit() {
    this.profiles = await this.userService.listUsers({})
      .then(response => response.users);

    this.rosterService.listRosterTypes({})
      .then(response => {
        this.rosterTypes = response.rosterTypes;
        this.cdr.markForCheck();
      })

    await this.loadRosters();

    this.cdr.markForCheck();
  }
}