import { Pipe, PipeTransform } from "@angular/core";
import { ConstraintViolationList } from "@tkd/apis";

@Pipe({
    name: 'constraintViolation',
    pure: true
})
export class TkdConstraintViolationPipe implements PipeTransform {
    transform(value: ConstraintViolationList): string[] {
        if (value.violations.length === 0) {
            return [];
        }

        return value.violations.map(val => {
          let prefix = '';
          let name: string | null | undefined = '';

          switch (val.kind.case) {
            case 'evaluation':
              prefix = 'Regel'
              name = val.kind.value.description
              break;

            case 'offTime':
              prefix = 'Abwesenheit'
              name = val.kind.value.entry?.description;
              break;
          }

          if (!name) {
            return prefix;
          }

          return `${prefix}: ${name}`
        })
    }
}
