import { DragDropModule } from "@angular/cdk/drag-drop";
import { OverlayModule } from "@angular/cdk/overlay";
import { CommonModule } from "@angular/common";
import { NgModule } from "@angular/core";
import { FormsModule, ReactiveFormsModule } from "@angular/forms";
import { RouterModule } from "@angular/router";
import { NgIconsModule } from "@ng-icons/core";
import { lucideAlertTriangle, lucideCalendar, lucideCheck, lucideCheckCircle, lucideCog, lucideDownload, lucideListPlus, lucideLoader2, lucidePencil, lucidePrinter, lucideRedo2, lucideSave, lucideSearch, lucideUndo2, lucideX } from '@ng-icons/lucide';
import { BrnAlertDialogModule } from '@spartan-ng/ui-alertdialog-brain';
import { BrnCommandModule } from '@spartan-ng/ui-command-brain';
import { BrnDialogModule } from '@spartan-ng/ui-dialog-brain';
import { BrnHoverCardModule } from '@spartan-ng/ui-hovercard-brain';
import { BrnMenuTriggerDirective } from '@spartan-ng/ui-menu-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { BrnSeparatorModule } from '@spartan-ng/ui-separator-brain';
import { BrnSheetModule } from '@spartan-ng/ui-sheet-brain';
import { BrnTableModule } from '@spartan-ng/ui-table-brain';
import { BrnTooltipModule } from '@spartan-ng/ui-tooltip-brain';
import { HlmAlertDialogModule } from '@tierklinik-dobersberg/angular/alertdialog';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { HlmBadgeModule } from "@tierklinik-dobersberg/angular/badge";
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { HlmCommandModule } from '@tierklinik-dobersberg/angular/command';
import { HlmDialogModule } from '@tierklinik-dobersberg/angular/dialog';
import { HlmHoverCardModule } from '@tierklinik-dobersberg/angular/hovercard';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { HlmMenuBarModule, HlmMenuModule } from '@tierklinik-dobersberg/angular/menu';
import { DisplayNamePipe, DurationPipe, TkdInListPipe, ToUserPipe, UserColorPipe, UserContrastColorPipe } from "@tierklinik-dobersberg/angular/pipes";
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { HlmSeparatorModule } from '@tierklinik-dobersberg/angular/separator';
import { HlmSheetModule } from "@tierklinik-dobersberg/angular/sheet";
import { HlmSkeletonModule } from '@tierklinik-dobersberg/angular/skeleton';
import { HlmSpinnerModule } from "@tierklinik-dobersberg/angular/spinner";
import { HlmSwitchModule } from '@tierklinik-dobersberg/angular/switch';
import { HlmTableModule } from '@tierklinik-dobersberg/angular/table';
import { HlmTooltipModule } from "@tierklinik-dobersberg/angular/tooltip";
import { HlmH1Directive, HlmH2Directive, HlmH3Directive } from '@tierklinik-dobersberg/angular/typography';
import { NzDatePickerModule } from "ng-zorro-antd/date-picker";
import { NZ_DATE_CONFIG } from "ng-zorro-antd/i18n";
import { NzToolTipModule } from "ng-zorro-antd/tooltip";
import { JoinListPipe, UserAvatarPipe, UserLetterPipe } from 'src/app/common/pipes';
import { TkdInViewportDirective } from "../common/in-viewport";
import { TkdTableSortColumnComponent } from "../common/table-sort";
import { AppHeaderOutletDirective } from "../header-outlet.directive";
import { ApprovalComponent } from './approval/approval.component';
import { TkdConstraintIsHardPipe, TkdConstraintViolationPipe } from "./constraint-violation-text.pipe";
import { TkdDebounceEventDirective } from "./debounce-event.directive";
import { GroupByISOWeekPipe } from "./group-by-isoweek.pipe";
import { TkdRosterOverviewComponent } from "./overview/overview.component";
import { TkdRosterPlannerComponent, TkdRosterPlannerDayComponent } from './planner';
import { TkdRosterPlannerSettingsComponent } from "./planner/settings";
import { TkdShiftSelectComponent } from "./planner/shift-select";
import { TkdRoster2Routing } from './roster2-routing.module';
import { RosterPlannerService } from "./planner/planner.service";

@NgModule({
  imports: [
    TkdRoster2Routing,
    DragDropModule,
    OverlayModule,
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    RouterModule,
    NzDatePickerModule,
    DurationPipe,
    ToUserPipe,
    DisplayNamePipe,
    UserColorPipe,
    UserContrastColorPipe,
    TkdInListPipe,
    NgIconsModule,
    UserLetterPipe,
    NzToolTipModule,
    HlmSkeletonModule,
    HlmSwitchModule,

    HlmButtonModule,
    HlmIconModule,
    HlmMenuModule,
    HlmAlertDialogModule,
    HlmDialogModule,
    HlmInputModule,
    HlmLabelModule,
    HlmSelectModule,
    HlmTableModule,
    HlmAvatarModule,
    HlmH1Directive,
    HlmH2Directive,
    HlmH3Directive,
    HlmBadgeModule,
    HlmSpinnerModule,
    HlmSheetModule,
    BrnSheetModule,
    BrnCommandModule,
    HlmCommandModule,
    BrnSeparatorModule,
    HlmSeparatorModule,
    HlmHoverCardModule,
    HlmMenuBarModule,
    BrnHoverCardModule,
    TkdInViewportDirective,
    HlmTooltipModule,
    BrnTooltipModule,

    BrnMenuTriggerDirective,
    BrnAlertDialogModule,
    BrnDialogModule,
    BrnSelectModule,
    BrnTableModule,
    TkdTableSortColumnComponent,
    GroupByISOWeekPipe,
    JoinListPipe,
    AppHeaderOutletDirective,
    UserAvatarPipe,
  ],
  declarations: [
    TkdRosterPlannerComponent,
    TkdRosterPlannerDayComponent,
    TkdConstraintViolationPipe,
    TkdDebounceEventDirective,
    TkdRosterOverviewComponent,
    TkdShiftSelectComponent,
    ApprovalComponent,
    TkdRosterPlannerSettingsComponent,
    TkdConstraintIsHardPipe,
  ],
  exports: [
    ToUserPipe,
    DisplayNamePipe,
    DurationPipe,
  ],
  providers: [
    { provide: NZ_DATE_CONFIG, useValue: { firstDayOfWeek: 1 } },
    ...provideIcons({
      lucideListPlus,
      lucideSearch,
      lucideCheck,
      lucideCog,
      lucideAlertTriangle,
      lucideCalendar,
      lucideCheckCircle,
      lucidePencil,
      lucideDownload,
      lucidePrinter,
      lucideSave,
      lucideUndo2,
      lucideRedo2,
      lucideX,
      lucideLoader2
    }),
    RosterPlannerService,
  ]
})
export class TkdRoster2Module { }
