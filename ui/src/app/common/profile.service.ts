import { computed } from "@angular/core";
import { injectCurrentProfile } from '@tierklinik-dobersberg/angular/behaviors';

export function injectCurrentUserIsAdmin() {
  const profile = injectCurrentProfile()

  return computed(() => {
    const current = profile();

    return current?.roles.some(r => r.id === 'roster_manager' || r.id === 'idm_superuser')
  })
}
