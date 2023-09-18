import { bootstrapApplication } from '@angular/platform-browser';
import { AppComponent } from './app/app.component';
import { appConfig } from './app/app.config';

import { registerLocaleData } from '@angular/common';
import de from '@angular/common/locales/de';

registerLocaleData(de);

bootstrapApplication(AppComponent, appConfig)
  .catch((err) => console.error(err));
