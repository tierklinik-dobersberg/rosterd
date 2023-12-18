import { ApplicationConfig, LOCALE_ID, importProvidersFrom } from '@angular/core';
import { provideRouter } from '@angular/router';

import { APP_BASE_HREF } from '@angular/common';
import { provideHttpClient } from '@angular/common/http';
import { provideAnimations } from '@angular/platform-browser/animations';
import { TkdConnectModule } from '@tierklinik-dobersberg/angular/connect';
import { TkdLayoutModule } from '@tierklinik-dobersberg/angular/layout';
import { NZ_DATE_CONFIG, NZ_I18N, de_DE } from 'ng-zorro-antd/i18n';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { environment } from 'src/environments/environment';
import { routes } from './app.routes';
import { NzModalModule } from 'ng-zorro-antd/modal';
import { provideIcons } from '@ng-icons/core';
import { heroBars4, heroUser, heroCog6Tooth, heroClock } from '@ng-icons/heroicons/outline';
import { heroUserMini, heroXMarkMini } from '@ng-icons/heroicons/mini';
import { ionCalendarOutline, ionAirplaneOutline, ionAlarmOutline } from '@ng-icons/ionicons';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes),
    provideAnimations(),
    provideHttpClient(),
    { provide: NZ_I18N, useValue: de_DE },
    { provide: LOCALE_ID, useValue: 'de'},
    { provide: APP_BASE_HREF, useValue: '/'},
    { provide: NZ_DATE_CONFIG, useValue: { firstDayOfWeek: 1 } },
    importProvidersFrom(NzMessageModule),
    importProvidersFrom(NzModalModule),
    importProvidersFrom(TkdConnectModule.forRoot(environment)),
    importProvidersFrom(TkdLayoutModule),
    provideIcons({ heroBars4, heroXMarkMini, heroUserMini, heroUser, ionCalendarOutline, ionAirplaneOutline, ionAlarmOutline, heroClock, heroCog6Tooth })
  ],
};
