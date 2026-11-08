import { apiRequest } from "./client";
import type { Review, ReviewDirection, ReviewEnvelope, ReviewQueue, ReviewRating } from "./types";

/** The words to review today, counting days in timeZone; see localTimeZone. */
export async function getReviewQueue(timeZone: string, signal?: AbortSignal): Promise<ReviewQueue> {
  return apiRequest<ReviewQueue>("GET", `/api/review-queue?timeZone=${encodeURIComponent(timeZone)}`, undefined, signal);
}

/** Records how well a word was remembered one way round and returns when that card comes back. */
export async function createReview(
  entryId: string,
  direction: ReviewDirection,
  rating: ReviewRating,
  timeZone: string,
): Promise<Review> {
  const { review } = await apiRequest<ReviewEnvelope>("POST", `/api/vocabulary/${encodeURIComponent(entryId)}/reviews`, {
    direction,
    rating,
    timeZone,
  });
  return review;
}
