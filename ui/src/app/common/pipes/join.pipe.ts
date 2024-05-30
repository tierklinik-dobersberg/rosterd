import { Pipe, PipeTransform } from '@angular/core';


@Pipe({
  name: 'joinLists',
  standalone: true,
  pure: true
})
export class JoinListPipe implements PipeTransform {
  transform(list1: string[], list2: string[]): string[] {
    const set = new Set(list1);

    list2.forEach(value => set.add(value))

    return Array.from(set);
  }
}
