import { ChangeDetectionStrategy, Component, OnInit, TrackByFunction, computed, inject, signal } from '@angular/core';
import { provideIcons } from '@ng-icons/core';
import { lucideArrowUpDown, lucideCheck, lucideEye, lucideMoreVertical, lucidePencil, lucideSend, lucideTrash2 } from '@ng-icons/lucide';
import { injectAuthService, injectRosterService, injectUserService } from '@tierklinik-dobersberg/angular/connect';
import { Profile, Role, Roster, RosterType } from '@tierklinik-dobersberg/apis';
import { NzMessageService } from 'ng-zorro-antd/message';
import { injectContainerSize } from 'src/app/common/container';
import { SortColumn } from 'src/app/common/table-sort';

interface RosterWithLink {
  roster: Roster;
  link: string;
}

const sortFunctions: Record<string, (a: Roster, b: Roster) => number> = {
  range: (a, b) => {
    const ad = {
      from: new Date(a.from).getTime(),
      to: new Date(a.to).getTime(),
    }

    const bd = {
      from: new Date(b.from).getTime(),
      to: new Date(b.to).getTime(),
    }

    const diffFrom = bd.from - ad.from;
    if (diffFrom !== 0) {
      return diffFrom;
    }

    return bd.to - ad.to;
  },
  type: (a, b) => {
    return b.rosterTypeName.localeCompare(a.rosterTypeName)
  },
  approval: (a, b) => {
    const aa = a.approved ? 1 : 0;
    const ab = b.approved ? 1 : 0;

    return ab - aa;
  },
  lastEdit: (a, b) => {
    const at = a.updatedAt!.toDate().getTime();
    const bt = b.updatedAt!.toDate().getTime();

    return bt - at;
  }
} as const;


@Component({
  selector: 'app-overview',
  templateUrl: './overview.component.html',
  styles: [
    `
    :host {
      @apply flex flex-col overflow-hidden flex-grow;
    }
    `
  ],
  providers: provideIcons({
    lucideEye,
    lucideSend,
    lucidePencil,
    lucideCheck,
    lucideTrash2,
    lucideMoreVertical,
    lucideArrowUpDown
  }),
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class TkdRosterOverviewComponent implements OnInit {
  private readonly rosterService = injectRosterService();
  private readonly userService = injectUserService();
  private readonly messageService = inject(NzMessageService)
  private readonly authService = injectAuthService();
  protected readonly container = injectContainerSize();

  private readonly _rosters = signal<RosterWithLink[]>([]);

  protected readonly _displayedColumns = computed(() => {
    const columns = [
      'range',
    ];

    if (this.container.width() > 300) {
      columns.push('type')
    }

    if (this.container.width() > 440) {
      columns.push('approval')
    }

    if (this.container.md()) {
      columns.push('editor')
    }

    if (this.container.sm()) {
      columns.push('lastEdit')
    }

    columns.push('actions')

    return columns;
  })

  protected readonly _sort = signal<SortColumn<typeof sortFunctions> | null>({
    column: 'range',
    direction: 'ASC'
  });

  protected readonly _sortedRosters = computed(() => {
    const sort = this._sort();
    const rosters = this._rosters();

    if (!sort) {
      return [...rosters];
    }

    const fn = sortFunctions[sort.column];

    return [...rosters]
      .sort((a, b) => {
        const result = fn(a.roster, b.roster);

        if (sort.direction === 'ASC') {
          return result * -1;
        }

        return result;
      })
  })

  profiles = signal<Profile[]>([]);
  rosterTypes = signal<RosterType[]>([]);
  isAdmin = signal(false);

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
    await this.rosterService
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
      .then(rosters => {
        this._rosters.set(rosters);
      })
  }

  async ngOnInit() {
    this.userService.listUsers({})
      .then(response => this.profiles.set(response.users));

    this.rosterService.listRosterTypes({})
      .then(response => {
        this.rosterTypes.set(response.rosterTypes);
      })


    this.authService
      .introspect({})
      .then(response => {
        this.isAdmin.set(response.profile!.roles!.find((r: Role) => ['idm_superuser', 'roster_manager'].includes(r.name)) !== undefined);
      });

    await this.loadRosters();
  }
}
