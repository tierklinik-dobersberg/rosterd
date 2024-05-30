import { Pipe, PipeTransform } from "@angular/core";
import { getWeek } from 'date-fns';

export interface WeekGroup {
  week: number;
  dates: Date[];
}

@Pipe({
  name: 'groupISOWeek',
  pure: true,
  standalone: true,
})
export class GroupByISOWeekPipe implements PipeTransform {
  transform(value: Date[]): WeekGroup[] {
    const result: WeekGroup[] = [];

    let lastISO: null | number = null;
    value.forEach(date => {
      const iso = getWeek(date, {
        weekStartsOn: 1,
      });

      if (lastISO === iso) {
        result[result.length - 1].dates.push(date)
      } else {
        result.push({
          week: iso,
          dates: [date],
        })
        lastISO = iso;
      }
    })

    return result;
  }
}
