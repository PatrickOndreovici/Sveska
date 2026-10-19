import { ApiError, apiRequest } from "./client";
import type { LogInRequest, SignUpRequest, User, UserEnvelope } from "./types";

export async function signUp(input: SignUpRequest): Promise<User> {
  const { user } = await apiRequest<UserEnvelope>("POST", "/api/auth/signup", input);
  return user;
}

export async function logIn(input: LogInRequest): Promise<User> {
  const { user } = await apiRequest<UserEnvelope>("POST", "/api/auth/login", input);
  return user;
}

export async function logOut(): Promise<void> {
  await apiRequest<undefined>("POST", "/api/auth/logout");
}

/** Returns the logged-in user, or null when there is no valid session. */
export async function fetchCurrentUser(): Promise<User | null> {
  try {
    const { user } = await apiRequest<UserEnvelope>("GET", "/api/me");
    return user;
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) return null;
    throw error;
  }
}
