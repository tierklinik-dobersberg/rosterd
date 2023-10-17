import { ApplicationConfig, LOCALE_ID, importProvidersFrom } from '@angular/core';
import { provideRouter } from '@angular/router';

import { APP_BASE_HREF } from '@angular/common';
import { provideAnimations } from '@angular/platform-browser/animations';
import { NZ_DATE_CONFIG, NZ_I18N, de_DE } from 'ng-zorro-antd/i18n';
import { NzMessageModule } from 'ng-zorro-antd/message';
import { routes } from './app.routes';
import { connectProviders } from './connect_clients';
import { provideHttpClient } from '@angular/common/http';
import { NzIconModule } from 'ng-zorro-antd/icon';

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
    ...connectProviders
  ],
};
