import { apiRequest } from "./client";
import type { Review, ReviewEnvelope, ReviewQueue, ReviewRating } from "./types";

/** The words to review today, counting days in timeZone; see localTimeZone. */
export async function getReviewQueue(timeZone: string, signal?: AbortSignal): Promise<ReviewQueue> {
  return apiRequest<ReviewQueue>("GET", `/api/review-queue?timeZone=${encodeURIComponent(timeZone)}`, undefined, signal);
}

/** Records how well a word was remembered and returns when it comes back. */
export async function createReview(entryId: string, rating: ReviewRating, timeZone: string): Promise<Review> {
  const { review } = await apiRequest<ReviewEnvelope>("POST", `/api/vocabulary/${encodeURIComponent(entryId)}/reviews`, {
    rating,
    timeZone,
  });
  return review;
}
