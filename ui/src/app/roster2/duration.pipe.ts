import { isDevMode, Pipe, PipeTransform } from '@angular/core';
import { PartialMessage, Duration as ProtoDuration } from '@bufbuild/protobuf';
import { DurationLayout, Duration as DurationUtil } from 'src/duration';

export type InputUnit = 'ns' | 'µs' | 'ms' | 's' | 'm' | 'h';

@Pipe({
  name: 'duration',
  pure: true
})
export class DurationPipe implements PipeTransform {
  transform(value: BigInt | string | number | DurationUtil | ProtoDuration | PartialMessage<ProtoDuration> | undefined, layout: DurationLayout = 'default', input: InputUnit = 's'): string {
    if (value === undefined || value === null) {
      return '';
    }

    if (value instanceof ProtoDuration || (typeof value === 'object' && Object.hasOwn(value, 'seconds'))) {
      value = (value as ProtoDuration).seconds;
      if (input !== 's') {
        throw new Error('invalid input type when Duration from @bufbuild/protobuf is passed');
      }
    }

    value = Number(value);

    let d: DurationUtil;
    switch (input) {
      case 'h':
        d = DurationUtil.hours(+value);
        break;
      case 'm':
        d = DurationUtil.minutes(+value);
        break;
      case 's':
        d = DurationUtil.seconds(+value);
        break;
      case 'ms':
        d = DurationUtil.milliseconds(+value);
        break;
      case 'µs':
        d = DurationUtil.microseconds(+value);
        break;
      case 'ns':
        d = DurationUtil.nanoseconds(+value);
        break;
      default:
        if (isDevMode()) {
          return 'WRONG_LAYOUT';
        }
        return '';
    }

    return d.format(layout);
  }
}
