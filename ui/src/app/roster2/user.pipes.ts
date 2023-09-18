import { Pipe, PipeTransform } from "@angular/core";
import { Profile, User } from "@tkd/apis";

export enum UserExtraKey {
  CalendarID = "calendarId",
  Color = "color",
}

export interface UserProfile extends Profile {
  user: User & {
    extra: {
      fields: {
        [UserExtraKey.CalendarID]?: {
          kind: {
            case: 'stringValue';
            value: string,
          }
        },
        [UserExtraKey.Color]?: {
          kind: {
            case: 'stringValue';
            value: string,
          }
        },
      }
    }
  }
}

export function getCalendarId(user: UserProfile | Profile): string | null {
  const prop = user.user?.extra?.fields[UserExtraKey.CalendarID];

  if (!prop || prop.kind.case !== 'stringValue') {
    return null
  }

  return prop.kind.value;
}

export function getUserColor(user: UserProfile | Profile): string | null {
  const prop = user.user?.extra?.fields[UserExtraKey.Color];

  if (!prop || prop.kind.case !== 'stringValue') {
    return null
  }

  return prop.kind.value;
}

export function parseColor(input: string): number[] {
  if (input.substr(0, 1) === '#') {
    const collen = (input.length - 1) / 3;
    const fact = [17, 1, 0.062272][collen - 1];
    return [
      Math.round(parseInt(input.substr(1, collen), 16) * fact),
      Math.round(parseInt(input.substr(1 + collen, collen), 16) * fact),
      Math.round(parseInt(input.substr(1 + 2 * collen, collen), 16) * fact),
    ];
  }

  return input
    .split('(')[1]
    .split(')')[0]
    .split(',')
    .map((x) => +x);
}

export function getContrastFontColor(bgColor: string | null): string {
  // if (red*0.299 + green*0.587 + blue*0.114) > 186 use #000000 else use #ffffff
  // based on https://stackoverflow.com/a/3943023

  if (bgColor === null) {
    return '#000000'
  }

  let col = bgColor;
  if (bgColor.startsWith('#') && bgColor.length > 7) {
    col = bgColor.slice(0, 7);
  }
  const [r, g, b] = parseColor(col);

  if (r * 0.299 + g * 0.587 + b * 0.114 > 186) {
    return '#000000';
  }

  return '#ffffff';
}

@Pipe({
    name: 'color',
    pure: true
})
export class UserColorPipe implements PipeTransform {
    transform(value: Profile, ...args: any[]) {
      return getUserColor(value)
    }
}

@Pipe({
    name: 'contrastColor',
    pure: true
})
export class UserContrastColorPipe implements PipeTransform {
    transform(value: Profile, ...args: any[]) {
      return getContrastFontColor(getUserColor(value))
    }
}
