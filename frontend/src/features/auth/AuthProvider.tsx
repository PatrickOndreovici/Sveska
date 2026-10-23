import { createContext, useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import * as authApi from "../../lib/api/auth";
import type { LogInRequest, SignUpRequest, User } from "../../lib/api/types";

export type AuthState =
  | { status: "loading"; user: null }
  | { status: "anonymous"; user: null }
  | { status: "authenticated"; user: User };

export type AuthContextValue = AuthState & {
  signUp: (input: SignUpRequest) => Promise<void>;
  logIn: (input: LogInRequest) => Promise<void>;
  logOut: () => Promise<void>;
  /** Shows a change the API has saved, such as a new name or picture. */
  updateUser: (user: User) => void;
};

export const AuthContext = createContext<AuthContextValue | null>(null);

const anonymous: AuthState = { status: "anonymous", user: null };

/**
 * Holds who is logged in. The session itself lives in an HttpOnly cookie the
 * frontend cannot read; this only mirrors what GET /api/me reports.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: "loading", user: null });

  useEffect(() => {
    let cancelled = false;
    authApi.fetchCurrentUser().then(
      (user) => {
        if (!cancelled) setState(user ? { status: "authenticated", user } : anonymous);
      },
      (error: unknown) => {
        console.error("could not load the current user", error);
        if (!cancelled) setState(anonymous);
      },
    );
    return () => {
      cancelled = true;
    };
  }, []);

  const signUp = useCallback(async (input: SignUpRequest) => {
    const user = await authApi.signUp(input);
    setState({ status: "authenticated", user });
  }, []);

  const logIn = useCallback(async (input: LogInRequest) => {
    const user = await authApi.logIn(input);
    setState({ status: "authenticated", user });
  }, []);

  const logOut = useCallback(async () => {
    await authApi.logOut();
    setState(anonymous);
  }, []);

  const updateUser = useCallback((user: User) => {
    setState({ status: "authenticated", user });
  }, []);

  const value = useMemo(
    () => ({ ...state, signUp, logIn, logOut, updateUser }),
    [state, signUp, logIn, logOut, updateUser],
  );
  return <AuthContext value={value}>{children}</AuthContext>;
}
