import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit, inject } from '@angular/core';
import { ActivatedRoute, NavigationEnd, Router, RouterModule, RouterOutlet } from '@angular/router';
import { NgIconsModule } from '@ng-icons/core';
import { moveInOutAnimation } from '@tierklinik-dobersberg/angular/animations';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { AUTH_SERVICE } from '@tierklinik-dobersberg/angular/connect';
import { HlmIconModule } from '@tierklinik-dobersberg/angular/icon';
import { LayoutService } from '@tierklinik-dobersberg/angular/layout';
import { Profile, Role } from '@tierklinik-dobersberg/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDrawerModule } from 'ng-zorro-antd/drawer';
import { NzIconModule } from 'ng-zorro-antd/icon';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { BehaviorSubject, filter, from, map, share } from 'rxjs';
import { environment } from 'src/environments/environment';
import { TkdContainerSizeDirective } from './common/container/container.directive';
import { TkdRoster2Module } from './roster2/roster2.module';
import { HlmToasterModule } from '@tierklinik-dobersberg/angular/sonner';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { UserLetterPipe } from 'src/app/common/pipes';
import { DevSizeOutlineComponent } from './size-outline/size-outline';

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
    HlmIconModule,
    HlmButtonModule,
    NzDrawerModule,
    TkdContainerSizeDirective,
    HlmToasterModule,
    HlmAvatarModule,
    UserLetterPipe,
    DevSizeOutlineComponent,
  ],
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  animations: [
    moveInOutAnimation,
  ]
})
export class AppComponent implements OnInit {
  protected readonly accountServer = environment.accountService;
  protected readonly mainApplication = environment.mainApplication;
  protected readonly router = inject(Router);
  protected readonly route = inject(ActivatedRoute)
  protected readonly cdr = inject(ChangeDetectorRef);
  protected readonly layout = inject(LayoutService).withAutoUpdate();

  drawerVisible = false;

  closeDrawer() {
    this.drawerVisible = false;
  }

  isRosterView = false;

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

        this.isRosterView = this.router.routerState.snapshot.url.startsWith('/roster/view')
          || this.router.routerState.snapshot.url.startsWith('/roster/plan');


        this.cdr.markForCheck();
      })
  }
}
