import { Signal, effect, signal } from "@angular/core";

export function debouncedSignal<T>(
  source: Signal<T>,
  debounceTimeInMs = 0,
) {
  const debounced = signal(source());
  effect(
    (onCleanup) => {
      const value = source();
      const timeout = setTimeout(() => debounced.set(value), debounceTimeInMs);

      onCleanup(() => clearTimeout(timeout));
    },
    {allowSignalWrites: true},
  );

  return debounced;
}
