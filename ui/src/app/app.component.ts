import { LayoutService } from '@tierklinik-dobersberg/angular/layout';
import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, inject } from '@angular/core';
import { NavigationEnd, Router, RouterModule, RouterOutlet } from '@angular/router';
import { AUTH_SERVICE } from '@tierklinik-dobersberg/angular/connect';
import { Profile, Role } from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDrawerModule } from 'ng-zorro-antd/drawer';
import { NzIconModule } from 'ng-zorro-antd/icon';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { BehaviorSubject, filter, from, map, share } from 'rxjs';
import { environment } from 'src/environments/environment';
import { TkdRoster2Module } from './roster2/roster2.module';
import { NgIconsModule } from '@ng-icons/core';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    RouterOutlet,
    NzAvatarModule,
    TkdRoster2Module,
    RouterModule,
    NzMessageModule,
    NzIconModule,
    NgIconsModule,
    NzDrawerModule,
  ],
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css'],
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class AppComponent implements OnInit {
  public readonly accountServer = environment.accountService;
  public readonly mainApplication = environment.mainApplication;
  public readonly router = inject(Router);
  public readonly cdr = inject(ChangeDetectorRef);
  public readonly layout = inject(LayoutService).withAutoUpdate();

  drawerVisible = false;

  closeDrawer() {
    this.drawerVisible = false;
  }

  profile = from(
    inject(AUTH_SERVICE).introspect({})
      .then(response => response.profile)
  ).pipe(
    share({connector: () => new BehaviorSubject<Profile | undefined>(undefined)}),
    filter(p => !!p),
  )

  isAdmin = this.profile
    .pipe(map(p => {
      if (p!.roles.find((role: Role) => ['idm_superuser', 'roster_manager'].includes(role.name))) {
        return true
      }

      return false
    }))

  ngOnInit() {
    this.router
      .events
      .pipe(
        filter(evt => evt instanceof NavigationEnd)
      )
      .subscribe(() => {
        this.drawerVisible = false;

        this.cdr.markForCheck();
      })
  }
}
