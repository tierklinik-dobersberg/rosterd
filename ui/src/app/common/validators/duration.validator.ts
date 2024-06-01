import { AbstractControl, NG_VALIDATORS, ValidationErrors, Validator, ValidatorFn } from '@angular/forms';
import { Directive } from '@angular/core';
import { Duration } from 'src/duration';

@Directive({
  standalone: true,
  // eslint-disable-next-line @angular-eslint/directive-selector
  selector: '[duration]',
  providers: [
    {
      provide: NG_VALIDATORS,
      useExisting: DurationValidatorDirective,
      multi: true,
    }
  ]
})
export class DurationValidatorDirective implements Validator {
  validate(control: AbstractControl<string>): ValidationErrors | null {
    return validateDuration(control);
  }
}

/**
 *
 * @param ctrl The {@link @angular/core#AbstractControl} that holds the value to validate.
 * @returns {@link @angular/core#ValidationErrors}
 */
export const validateDuration: ValidatorFn = (ctrl: AbstractControl<string>) => {
  if (ctrl.value === '') {
    return null;
  }

  if (typeof ctrl.value !== 'string') {
    return null;
  }

  try {
    const duration = Duration.parseString(ctrl.value, true)

    if (duration.seconds === 0) {
      return {'required': true}
    }

    return null;
  } catch (err) {
    return {
      'durationFormat': err,
    }
  }
}
