import { ChangeDetectionStrategy, Component, ElementRef, ViewChild, computed, effect, inject, model } from "@angular/core";
import { injectComputedFilterSheetSide } from "@tierklinik-dobersberg/angular/behaviors";
import { RosterPlannerService } from "../planner.service";

@Component({
  // eslint-disable-next-line @angular-eslint/component-selector
  selector: 'tkd-planner-settings',
  exportAs: 'plannerSettings',
  templateUrl: './settings.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TkdRosterPlannerSettingsComponent {
  private readonly _service = inject(RosterPlannerService);

  protected readonly _computedSheetSide = injectComputedFilterSheetSide();

  protected readonly _shifts = computed(() => {
    return Array.from(this._service.sessionState().shiftDefinitions.values())
  })
  protected readonly _shiftsToShow = model<string[]>([]);

  @ViewChild('button', { static: true, read: ElementRef })
  protected button!: ElementRef<HTMLButtonElement>;

  constructor() {
    effect(() => {
      const shiftsToShow = this._service.settings().shiftIdsToShow;
      this._shiftsToShow.set(shiftsToShow);
    }, {
      allowSignalWrites: true,
    })
  }

  reset() {
    this._shiftsToShow.set([]);

    this.apply();
  }

  apply() {
    this._service.settings.update((current) => {
      const copy = {...current};
      copy.shiftIdsToShow = this._shiftsToShow();
      return copy
    })
  }

  open() {
    this.button.nativeElement.click();
  }
}
