import { DestroyRef, Directive, Injectable, TemplateRef, inject, signal } from '@angular/core';

@Directive({
  selector: '[appHeader]',
  standalone: true,
})
export class AppHeaderOutletDirective {
  public readonly template = inject(TemplateRef);

  constructor() {
    inject(AppHeaderOutletService)
      .setOutlet(this.template);
  }
}

@Injectable()
export class AppHeaderOutletService {
  private readonly _outlet = signal<TemplateRef<unknown> | null>(null);

  outlet = this._outlet.asReadonly();

  setOutlet(t: TemplateRef<unknown>) {
    this._outlet.set(t);

    inject(DestroyRef)
      .onDestroy(() => this._outlet.set(null));
  }
}
