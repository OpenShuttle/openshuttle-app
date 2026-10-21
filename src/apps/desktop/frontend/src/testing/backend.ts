import { setTransport } from '@wailsio/runtime';
import * as Bindings from '@bindings/apps/desktop';

/** Answers a binding call: a value, or a function of the call's arguments (it may throw a backend error). */
type Answer = unknown | ((...args: never[]) => unknown);

/** The Wails runtime object a binding call goes to (objectNames.Call). */
const CALL_OBJECT = 0;

/** Maps the method IDs of the generated bindings to "Service.Method" by calling each one once. */
function bindingNames(): Map<number, string> {
  const names = new Map<number, string>();
  let captured = -1;
  setTransport({
    call: (_object: number, _method: number, _window: string, args: { methodID?: number }) => {
      captured = args?.methodID ?? -1;
      return Promise.resolve(null);
    },
  });
  for (const [service, namespace] of Object.entries(Bindings)) {
    if (typeof namespace !== 'object' || namespace === null) {
      continue;
    }
    for (const [method, fn] of Object.entries(namespace as Record<string, unknown>)) {
      if (typeof fn !== 'function') {
        continue;
      }
      captured = -1;
      const p = (fn as () => Promise<unknown>)();
      p?.catch?.(() => undefined);
      if (captured >= 0) {
        names.set(captured, `${service}.${method}`);
      }
    }
  }
  return names;
}

let names: Map<number, string> | null = null;

/** A Go error as the app receives it: a stable code in the cause. */
export function backendFailure(code: string, set?: number): Error {
  return Object.assign(new Error(code), { cause: set ? { code, set } : { code } });
}

/**
 * Go, faked for component tests: every binding call ("TournamentService.CallMatch")
 * gets the answer set with `on`, null otherwise, and is recorded.
 */
export class FakeBackend {
  readonly calls: { name: string; args: unknown[] }[] = [];
  private readonly answers = new Map<string, Answer>();

  constructor() {
    names ??= bindingNames();
    setTransport({
      call: async (object: number, _method: number, _window: string, args: { methodID?: number; args?: unknown[] }) => {
        if (object !== CALL_OBJECT || args?.methodID === undefined) {
          return null; // window, screens, events: nothing to fake
        }
        const name = names!.get(args.methodID) ?? `#${args.methodID}`;
        const callArgs = args.args ?? [];
        this.calls.push({ name, args: callArgs });
        const answer = this.answers.get(name);
        const value = typeof answer === 'function' ? (answer as (...a: unknown[]) => unknown)(...callArgs) : answer;
        return structuredClone(value ?? null);
      },
    });
  }

  /** Sets what a binding answers, e.g. on('TournamentService.GetControlDeskView', desk). */
  on(name: string, answer: Answer): this {
    this.answers.set(name, answer);
    return this;
  }

  /** The arguments of each call of a binding. */
  argsOf(name: string): unknown[][] {
    return this.calls.filter((c) => c.name === name).map((c) => c.args);
  }

  /** Whether a binding was called. */
  called(name: string): boolean {
    return this.calls.some((c) => c.name === name);
  }

  /** Emits a Wails event to the frontend, as Go does. */
  emit(name: string, data: unknown = null): void {
    (window as unknown as { _wails: { dispatchWailsEvent: (e: unknown) => void } })._wails.dispatchWailsEvent({ name, data });
  }
}
