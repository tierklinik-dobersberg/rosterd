import { ApplicationConfig, LOCALE_ID, importProvidersFrom } from '@angular/core';
import { provideRouter } from '@angular/router';

import { APP_BASE_HREF } from '@angular/common';
import { provideHttpClient } from '@angular/common/http';
import { provideAnimations } from '@angular/platform-browser/animations';
import { provideIcons } from '@ng-icons/core';
import { heroUserMini, heroXMarkMini } from '@ng-icons/heroicons/mini';
import { heroBars4, heroClock, heroCog6Tooth, heroUser } from '@ng-icons/heroicons/outline';
import { ionAirplaneOutline, ionAlarmOutline, ionCalendarOutline } from '@ng-icons/ionicons';
import { lucideCalendar, lucideCalendarClock, lucideCog, lucideMenu, lucidePlane, lucideUserX } from '@ng-icons/lucide';
import { TkdConnectModule } from '@tierklinik-dobersberg/angular/connect';
import { TkdLayoutModule, provideBreakpoints } from '@tierklinik-dobersberg/angular/layout';
import { Breakpoints } from '@tierklinik-dobersberg/tailwind/breakpoints';
import { NZ_DATE_CONFIG, NZ_I18N, de_DE } from 'ng-zorro-antd/i18n';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { NzModalModule } from 'ng-zorro-antd/modal';
import { environment } from 'src/environments/environment';
import { routes } from './app.routes';
import { AppHeaderOutletService } from './header-outlet.directive';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes),
    provideAnimations(),
    provideHttpClient(),
    { provide: NZ_I18N, useValue: de_DE },
    { provide: LOCALE_ID, useValue: 'de' },
    { provide: APP_BASE_HREF, useValue: '/' },
    { provide: NZ_DATE_CONFIG, useValue: { firstDayOfWeek: 1 } },
    importProvidersFrom(NzMessageModule),
    importProvidersFrom(NzModalModule),
    importProvidersFrom(TkdConnectModule.forRoot(environment)),
    importProvidersFrom(TkdLayoutModule),
    provideIcons({
      heroBars4, heroXMarkMini, heroUserMini, heroUser, ionCalendarOutline, ionAirplaneOutline, ionAlarmOutline, heroClock, heroCog6Tooth, lucideMenu,
      lucideCalendar,
      lucidePlane,
      lucideCalendarClock,
      lucideUserX,
      lucideCog
    }),
    provideBreakpoints(Breakpoints),
    AppHeaderOutletService,
  ],
};
