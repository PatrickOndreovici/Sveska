import { apiRequest } from "./client";
import type { ProfileInput, User, UserEnvelope } from "./types";

/** The largest picture the API accepts; checked here too so a big file fails before it uploads. */
export const MAX_AVATAR_BYTES = 10 * 1024 * 1024;

/** The picture formats the API reads, for the file picker. */
export const AVATAR_TYPES = "image/jpeg,image/png,image/gif,image/webp";

export async function updateProfile(input: ProfileInput): Promise<User> {
  const { user } = await apiRequest<UserEnvelope>("PUT", "/api/me", input);
  return user;
}

/** Sets the profile picture and returns the user with its new avatarUrl. */
export async function uploadAvatar(file: File): Promise<User> {
  const form = new FormData();
  form.append("file", file);
  const { user } = await apiRequest<UserEnvelope>("PUT", "/api/me/avatar", form);
  return user;
}

export async function deleteAvatar(): Promise<void> {
  await apiRequest<undefined>("DELETE", "/api/me/avatar");
}
