import { toast } from 'ngx-sonner';
import { Injectable, computed, signal } from "@angular/core";
import { ConnectError } from "@connectrpc/connect";
import { injectAuthService, injectUserService } from "@tierklinik-dobersberg/angular/connect";
import { Profile } from "@tierklinik-dobersberg/apis/idm/v1";
import { interval, retry, startWith, switchMap } from 'rxjs';

@Injectable({providedIn: 'root'})
export class ProfileService {
  private readonly authService = injectAuthService();
  private readonly userService = injectUserService();

  private _profiles = signal<Profile[]>([]);
  public profiles = this._profiles.asReadonly();

  private _current = signal<Profile | null>(null)
  public current = this._current.asReadonly();

  public isAdmin = computed(() => {
    const current = this.current();
    if (!current) {
      return false
    }

    if (current.roles.find(role => role.id === 'roster_manager' || role.id === 'idm_superuser')) {
      return true
    }

    return false;
  })

  constructor() {
    this.authService
      .introspect({
        excludeFields: true,
        readMask: {
          paths: ['profile.user.avatar']
        }
      })
      .then(response => this._current.set(response.profile!))
      .catch(err => {
        const connectErr = ConnectError.from(err);
        toast.error('Fehler beim laden des Benutzerprofiles', {
          description: connectErr.message
        })
      })

    interval(5 * 60 * 1000)
      .pipe(
        startWith(-1),
        switchMap(() => this.userService.listUsers({
          excludeFields: true,
          fieldMask: {
            paths: ['users.user.avatar']
          }
        })),
        retry({
          delay: 1000,
        })
      )
      .subscribe(result => {
        this._profiles.set(result.users);
      })
  }
}
