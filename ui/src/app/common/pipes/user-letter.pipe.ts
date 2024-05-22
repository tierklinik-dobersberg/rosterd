import { Profile } from '@tierklinik-dobersberg/apis';
import { Pipe, PipeTransform } from "@angular/core";

@Pipe({
  name: 'userLetter',
  pure: true,
  standalone: true,
})
export class UserLetterPipe implements PipeTransform {
  transform(input: Profile): string {
    if (input.user?.displayName) {
      if (input.user.displayName.includes(" ")) {
        const parts = input.user.displayName.split(" ");
        if (parts[0].length > 0 && parts[1].length > 0) {
          return parts[0][0].toLocaleUpperCase() + parts[1][0].toLocaleUpperCase();
        }

        return input.user.displayName.substring(0, 2).toLocaleUpperCase();
      }
    }

    let name = "";

    if (input.user?.firstName) {
      name += input.user.firstName[0];
    }

    if (input.user?.lastName) {
      name += input.user.lastName[0];
    }

    if (name === "") {
      name = input.user!.username.substring(0, 2);
    }

    return name.toLocaleUpperCase();
  }
}
