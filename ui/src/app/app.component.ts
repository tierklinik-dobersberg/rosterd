import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { ActivatedRoute, NavigationEnd, Router, RouterModule, RouterOutlet } from '@angular/router';
import { NgIconsModule } from '@ng-icons/core';
import { moveInOutAnimation } from '@tierklinik-dobersberg/angular/animations';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { HlmIconModule } from '@tierklinik-dobersberg/angular/icon';
import { LayoutService } from '@tierklinik-dobersberg/angular/layout';
import { HlmToasterModule } from '@tierklinik-dobersberg/angular/sonner';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzDrawerModule } from 'ng-zorro-antd/drawer';
import { NzIconModule } from 'ng-zorro-antd/icon';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { filter } from 'rxjs';
import { UserAvatarPipe, UserLetterPipe } from 'src/app/common/pipes';
import { environment } from 'src/environments/environment';
import { TkdContainerSizeDirective } from './common/container/container.directive';
import { ProfileService } from './common/profile.service';
import { AppHeaderOutletDirective, AppHeaderOutletService } from './header-outlet.directive';
import { TkdRoster2Module } from './roster2/roster2.module';
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
    AppHeaderOutletDirective,
    UserAvatarPipe,
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
  protected readonly layout = inject(LayoutService);
  protected readonly profileService = inject(ProfileService);

  protected readonly _outlet = (() => {
    const service = inject(AppHeaderOutletService);

    return computed(() => service.outlet());
  })();

  drawerVisible = false;

  closeDrawer() {
    this.drawerVisible = false;
  }

  protected readonly _isRosterView = signal(false);
  protected readonly _profile = computed(() => this.profileService.current())
  protected readonly _isAdmin = computed(() => this.profileService.isAdmin())

  ngOnInit() {
    this.router
      .events
      .pipe(
        filter(evt => evt instanceof NavigationEnd)
      )
      .subscribe(() => {
        this.drawerVisible = false;

        this._isRosterView.set(
          this.router.routerState.snapshot.url.startsWith('/roster/view')
          || this.router.routerState.snapshot.url.startsWith('/roster/plan')
        );
      })
  }
}
