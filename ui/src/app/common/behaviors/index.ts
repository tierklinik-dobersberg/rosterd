import { inject, computed, Signal } from '@angular/core';
import { ProfileService } from '../profile.service';
import { Profile } from '@tierklinik-dobersberg/apis';

export * from './filter-sheet-side';
export * from './debounced-signal';
export * from './sorting';

export function injectUserProfiles(): Signal<Profile[]> {
  const service = inject(ProfileService);

  return computed(() => service.profiles())
}
