import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, isDevMode, signal } from "@angular/core";
import { HlmBadgeModule } from "@tierklinik-dobersberg/angular/badge";
import { LayoutService } from "@tierklinik-dobersberg/angular/layout";
import { injectContainerSize } from "../common/container";

@Component({
  selector: 'app-dev-size',
  imports: [
    HlmBadgeModule
  ],
  template: `
    @if (isDevMode) {
      <span hlmBadge variant="outline" class="rounded-sm border-green-600 block w-full">Layout: {{ _computedLayoutBreakpoint() }}</span>
      <span hlmBadge variant="outline" class="rounded-sm border-green-600 block w-full">Container: {{ _computedContainerBreakpoint() }}</span>
    }
  `,
  styles: [
    `
    :host {
      @apply flex-col justify-start items-start absolute bottom-1 right-1 w-fit gap-1;
    }
    `
  ],
  // eslint-disable-next-line @angular-eslint/no-host-metadata-property
  host: {
    '[style.display]': 'isDevMode ? "flex" : "none"'
  },
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
})
export class DevSizeOutlineComponent {
  protected readonly layout = inject(LayoutService);
  protected readonly container = injectContainerSize();

  protected readonly isDevMode = isDevMode();

  private readonly _windowSize = signal<number>(0);

  constructor() {
    if (!this.isDevMode) {
      return;
    }

    const body: HTMLBodyElement = document.getElementsByTagName("body")[0];
    const observer = new ResizeObserver(() => {
      const rect = body.getBoundingClientRect();
      this._windowSize.set(rect.width)
    })

    observer.observe(body);

    inject(DestroyRef)
      .onDestroy(() => observer.disconnect());
  }

  protected readonly _computedLayoutBreakpoint = computed(() => {
    let bp = 'xs';

    if (this.layout.sm()) {
      bp = 'sm'
    }

    if (this.layout.md()) {
      bp = 'md'
    }

    if (this.layout.lg()) {
      bp = 'lg'
    }

    if (this.layout.xl()) {
      bp = 'xl'
    }

    if (this.layout.xxl()) {
      bp = 'xxl'
    }

    return bp + ` (${this._windowSize()}px)`
  })


  protected readonly _computedContainerBreakpoint = computed(() => {
    let bp = 'xs';

    if (this.container.sm()) {
      bp = 'sm'
    }

    if (this.container.md()) {
      bp = 'md'
    }

    if (this.container.lg()) {
      bp = 'lg'
    }

    if (this.container.xl()) {
      bp = 'xl'
    }

    if (this.container.xxl()) {
      bp = 'xxl'
    }

    return bp + ` (${this.container.width()}px)`
  })
}
