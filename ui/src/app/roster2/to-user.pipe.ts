import { Pipe, PipeTransform } from "@angular/core";
import { Profile } from "@tkd/apis";

@Pipe({
  name: 'toUser',
  pure: true,
})
export class ToUserPipe implements PipeTransform {
  transform(idOrProfile: Profile | string, profiles: Profile[]): Profile | null {
    if (idOrProfile instanceof Profile) {
      return idOrProfile;
    }

    return profiles.find(p => p.user?.id === idOrProfile) || null;
  }
}

@Pipe({
  name: "displayName",
  pure: true,
})
export class DisplayNamePipe implements PipeTransform {
  transform(value: Profile | null, ...args: any[]) {
      if (!value) {
        return ''
      }

      if (value.user?.displayName) {
        return value.user.displayName;
      }

      if (value.user?.username) {
        return value.user.username
      }

      return '';
  }
}
