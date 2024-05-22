import { DestroyRef, Directive, ElementRef, computed, effect, inject, input, signal } from '@angular/core';
import { hlm } from '@spartan-ng/ui-core';
import { Breakpoints } from '@tierklinik-dobersberg/tailwind/breakpoints';
import { clsx } from 'clsx';

export function injectContainerSize(): TkdContainerSizeDirective {
  return inject(TkdContainerSizeDirective, { skipSelf: true })
}

@Directive({
  // eslint-disable-next-line @angular-eslint/directive-selector
  selector: '[containerSize]',
  exportAs: "containerSize",
  standalone: true,
})
export class TkdContainerSizeDirective {
  protected readonly element = inject(ElementRef);
  private readonly observer: ResizeObserver;

  private readonly limits = {
    sm: parseInt(Breakpoints.sm),
    md: parseInt(Breakpoints.md),
    lg: parseInt(Breakpoints.lg),
    xl: parseInt(Breakpoints.xl),
    xxl: parseInt(Breakpoints['2xl']),
  } as const;

  public readonly sm = signal<boolean>(false);
  public readonly md = signal<boolean>(false);
  public readonly lg = signal<boolean>(false);
  public readonly xl = signal<boolean>(false);
  public readonly xxl = signal<boolean>(false);
  public readonly width = signal<number>(0);

  constructor() {
    this.observer = new ResizeObserver(() => this.update());

    inject(DestroyRef)
      .onDestroy(() => this.observer.disconnect());

    this.observer.observe(this.element.nativeElement)

    effect(() => {
      // for what ever reason we need to call this ...
      this.width();
    })
  }

  private update() {
    const elem: HTMLElement = this.element.nativeElement;
    const width = elem.getBoundingClientRect().width;


    this.sm.set(width >= this.limits.sm);
    this.md.set(width >= this.limits.md);
    this.lg.set(width >= this.limits.lg);
    this.xl.set(width >= this.limits.xl);
    this.xxl.set(width >= this.limits.xxl);
    this.width.set(width);
  }
}

@Directive({
  // eslint-disable-next-line @angular-eslint/directive-selector
  selector: '[sizeClass], [sizeClass.sm], [sizeClass.md], [sizeClass.lg], [sizeClass.xl], [sizeClass.xxl]',
  exportAs: 'sizeClass',
  standalone: true,
  // eslint-disable-next-line @angular-eslint/no-host-metadata-property
  host: {
    '[class]': '_computedClass()',
  },
})
export class TkdContainerSizeClassDirective {
  private readonly container = injectContainerSize();

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly sm = input<any>(null, { alias: 'sizeClass.sm' });
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly md = input<any>(null, { alias: 'sizeClass.md' });
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly lg = input<any>(null, { alias: 'sizeClass.lg' });
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly xl = input<any>(null, { alias: 'sizeClass.xl' });
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly xxl = input<any>(null, { alias: 'sizeClass.xxl' });

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  readonly classes = input<{ [key: string]: any } | null>(null, { alias: "sizeClass" })

  // Returns the internal computed class signal for use in other component templates.
  public get cls() {
    return this._computedClass;
  }

  protected readonly _computedClass = computed(() => {
    const cls = this.classes();
    const csm = this.sm();
    const cmd = this.md();
    const clg = this.lg();
    const cxl = this.xl();
    const cxxl = this.xxl();

    const width = this.container.width();
    const sm = this.container.sm();
    const md = this.container.md();
    const lg = this.container.lg();
    const xl = this.container.xl();
    const xxl = this.container.xxl();

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const classes: any[] = [];

    // default classes that always apply
    classes.push(cls?.['']);

    if (sm) {
      classes.push(csm, cls?.sm)
    }
    if (md) {
      classes.push(cmd, cls?.md)
    }
    if (lg) {
      classes.push(clg, cls?.lg)
    }
    if (xl) {
      classes.push(cxl, cls?.xl)
    }
    if (xxl) {
      classes.push(cxxl, cls?.xxl);
    }

    Object.keys(cls || {})
      .forEach(key => {

        const num = parseInt(key);
        if (isNaN(num)) {
          return
        }

        if (width >= num) {
          classes.push(cls?.[key])
        }
      })


    const result = hlm(clsx(classes));

    console.log(result, classes, cls, width)
    return result;
  })
}
