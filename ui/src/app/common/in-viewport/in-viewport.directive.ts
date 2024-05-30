import { DestroyRef, Directive, ElementRef, inject, signal } from "@angular/core";

@Directive({
  // eslint-disable-next-line @angular-eslint/directive-selector
  selector: '[inView]',
  exportAs: 'inView',
  standalone: true,
})
export class TkdInViewportDirective {
  private readonly _inView = signal(false);
  private readonly _element = inject(ElementRef);

  public readonly visible = this._inView.asReadonly();

  constructor() {
    const intersectionObserver = new IntersectionObserver(
      (entries) => {
        // Sometimes entries receive multiple entries
        // Last one is correct
        this._inView.set(entries[entries.length - 1].isIntersecting);
        console.log("element is in view: ", this.visible(), this._element.nativeElement, entries)
      },
      {
        threshold: 1,
      },
    );

    inject(DestroyRef)
      .onDestroy(() => intersectionObserver.disconnect());

    intersectionObserver.observe(this._element.nativeElement);
  }
}
