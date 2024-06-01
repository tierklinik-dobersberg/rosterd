import { Pipe, PipeTransform, inject } from "@angular/core";
import { DomSanitizer } from "@angular/platform-browser";
import { CONNECT_CONFIG } from "@tierklinik-dobersberg/angular/connect";
import { Profile } from "@tierklinik-dobersberg/apis";

@Pipe({
  standalone: true,
  name: 'avatar'
})
export class UserAvatarPipe implements PipeTransform {
  private readonly accountService = inject(CONNECT_CONFIG).accountService;
  private readonly sanatizer = inject(DomSanitizer);

  transform(value: Profile | string) {
    const id = typeof value === 'object' ? value.user!.id : value;

    return this.sanatizer.bypassSecurityTrustResourceUrl(`${this.accountService}/avatar/${id}`);
  }
}
