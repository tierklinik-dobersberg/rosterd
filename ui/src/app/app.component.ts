import { CommonModule } from '@angular/common';
import { Component, inject } from '@angular/core';
import { RouterModule, RouterOutlet } from '@angular/router';
import { Profile, Role } from '@tkd/apis';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { BehaviorSubject, filter, from, map, share } from 'rxjs';
import { AUTH_SERVICE } from './connect_clients';
import { TkdRoster2Module } from './roster2/roster2.module';
import { NzIconModule } from 'ng-zorro-antd/icon';

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
  ],
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css']
})
export class AppComponent {
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
}
