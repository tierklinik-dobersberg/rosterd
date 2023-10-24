import { InjectionToken, Provider } from "@angular/core";
import { ActivatedRoute, Router } from "@angular/router";
import { Code, ConnectError, Interceptor, PromiseClient, Transport, createPromiseClient } from "@bufbuild/connect";
import { createConnectTransport } from "@bufbuild/connect-web";
import { AuthService, CalendarService, ConstraintService, HolidayService, OffTimeService, RoleService, RosterService, UserService, WorkShiftService, WorkTimeService } from "@tkd/apis";
import { NzMessageService } from "ng-zorro-antd/message";
import { environment } from "src/environments/environment";

export type AuthServiceClient = PromiseClient<typeof AuthService>;
export type UserServiceClient = PromiseClient<typeof UserService>;
export type CalendarServiceClient = PromiseClient<typeof CalendarService>;
export type RosterServiceClient = PromiseClient<typeof RosterService>;
export type WorkShiftServiceClient = PromiseClient<typeof WorkShiftService>;
export type OffTimeServiceClient = PromiseClient<typeof OffTimeService>;
export type WorkTimeServiceClient = PromiseClient<typeof WorkTimeService>;
export type HolidayServiceClient = PromiseClient<typeof HolidayService>;
export type ConstraintServiceClient = PromiseClient<typeof ConstraintService>;
export type RolesServiceClient = PromiseClient<typeof RoleService>;

export const AUTH_SERVICE = new InjectionToken<AuthServiceClient>('AUTH_SERVICE');
export const USER_SERVICE = new InjectionToken<UserServiceClient>('USER_SERVICE');
export const CALENDAR_SERVICE = new InjectionToken<CalendarServiceClient>('CALENDAR_SERVICE');
export const HOLIDAY_SERVICE = new InjectionToken<HolidayServiceClient>('HOLIDAY_SERVICE');
export const ROSTER_SERVICE = new InjectionToken<RosterServiceClient>('ROSTER_SERVICE');
export const OFFTIME_SERVICE = new InjectionToken<OffTimeServiceClient>('OFFTIME_SERVICE');
export const WORKTIME_SERVICE = new InjectionToken<WorkTimeServiceClient>('WORKTIME_SERVICE');
export const WORKSHIFT_SERVICE = new InjectionToken<WorkShiftServiceClient>('WORKSHIFT_SERVICE');
export const CONSTRAINT_SERVICE = new InjectionToken<ConstraintServiceClient>('CONSTRAINT_SERVICE');
export const ROLES_SERVICE = new InjectionToken<RolesServiceClient>('ROLES_SERVICE');


function serviceClientFactory(type: any, ep: string): (route: ActivatedRoute, router: Router, msgSvc: NzMessageService) => any {
  return ((route: ActivatedRoute, router: Router, msgSvc: NzMessageService) => {
    let transport = transportFactory(route, router, ep, [
      showErrorInterceptor(msgSvc),
    ]);

    return createPromiseClient(type, transport);
  });
}

function makeProvider(token: InjectionToken<any>, type: any, ep: string): Provider {
  return {
    deps: [
      ActivatedRoute,
      Router,
      NzMessageService,
    ],
    provide: token,
    useFactory: serviceClientFactory(type, ep),
  }
}

export const connectProviders: Provider[] = [
  makeProvider(AUTH_SERVICE, AuthService, environment.accountService) ,
  makeProvider(USER_SERVICE, UserService, environment.accountService),
  makeProvider(ROLES_SERVICE, RoleService, environment.accountService),
  makeProvider(CALENDAR_SERVICE, CalendarService, environment.calendarService),
  makeProvider(ROSTER_SERVICE, RosterService, environment.rosterService),
  makeProvider(OFFTIME_SERVICE, OffTimeService, environment.rosterService),
  makeProvider(WORKTIME_SERVICE, WorkTimeService, environment.rosterService),
  makeProvider(WORKSHIFT_SERVICE, WorkShiftService, environment.rosterService),
  makeProvider(CONSTRAINT_SERVICE, ConstraintService, environment.rosterService),
  makeProvider(HOLIDAY_SERVICE, HolidayService, environment.calendarService),
]

const retryRefreshToken: (transport: Transport, activatedRoute: ActivatedRoute, router: Router) => Interceptor = (transport, activatedRoute, router) => {
  let pendingRefresh: Promise<void> | null = null;

  return (next) => async (req) => {
    try {
      const result = await next(req)
      return result;

    } catch (err) {
      const connectErr = ConnectError.from(err);

      // don't retry the request if it was a Login or RefreshToken.
      if (req.service.typeName === AuthService.typeName && (req.method.name === 'Login' || req.method.name == 'RefreshToken')) {
        console.log("skipping retry as requested service is " + `${req.service.typeName}/${req.method.name}`)

        throw err
      }

      if (connectErr.code === Code.Unauthenticated) {
        if (pendingRefresh === null) {
          let _resolve: any;
          let _reject: any;
          pendingRefresh = new Promise((resolve, reject) => {
            _resolve = resolve;
            _reject = reject;
          })

          pendingRefresh
            .catch(() => {})
            .then(() => pendingRefresh = null)

          const cli = createPromiseClient(AuthService, transport);

          console.log(`[DEBUG] call to ${req.service.typeName}/${req.method.name} not authenticated, trying to refresh token`)
          try {
            let redirect = activatedRoute.snapshot.queryParamMap.get("redirect");
            if (!redirect && router.getCurrentNavigation() !== null) {
              redirect = router.getCurrentNavigation()!.extractedUrl.queryParamMap.get("redirect")
            }

            const res = await cli.refreshToken({
              requestedRedirect: redirect || '',
            })

            _resolve();
          } catch (refreshErr) {
            console.error("failed to refresh token", refreshErr)

            _reject(err);

            throw err;
          }
        } else {
          // wait for the pending refresh to finish
          try {
            await pendingRefresh;
          } catch (_) {
            throw err;
          }
        }

        // retry with a new access token.
        return await next(req);
      }

      throw err;
    }
  }
}

function showErrorInterceptor(nzMessageService: NzMessageService): Interceptor {
  return (next) => async req => {
    try {
      return await next(req)
    } catch (err) {
      nzMessageService?.error(ConnectError.from(err).rawMessage);

      throw err
    }
  }
}

export function transportFactory(route: ActivatedRoute, router: Router, endpoint: string, interceptors?: Interceptor[]): Transport {
  const retryTransport = createConnectTransport({baseUrl: environment.accountService, credentials: 'include'})

  return createConnectTransport({
    baseUrl: endpoint,
    credentials: 'include',
    jsonOptions: {
      ignoreUnknownFields: true
    },
    interceptors: [
      ...(interceptors || []),
      retryRefreshToken(retryTransport, route, router),
    ],
  })
}
