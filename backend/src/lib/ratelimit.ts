/**
 * Production-grade rate limiting
 * Prevents abuse and ensures fair resource usage
 */

import { logger } from "./logger";

interface RateLimitConfig {
  windowMs: number;      // Time window in milliseconds
  maxRequests: number;   // Max requests per window
  keyPrefix?: string;    // Redis key prefix (for distributed rate limiting)
}

interface RateLimitEntry {
  count: number;
  resetTime: number;
}

// In-memory store (use Redis in production with multiple instances)
const rateLimitStore = new Map<string, RateLimitEntry>();

// Default configurations for different endpoints
export const rateLimitConfigs = {
  // Strict limit for message creation (expensive operation)
  messageCreate: {
    windowMs: 60 * 1000,      // 1 minute
    maxRequests: 30,          // 30 messages per minute
  },
  // Moderate limit for session operations
  session: {
    windowMs: 60 * 1000,
    maxRequests: 60,
  },
  // Lenient limit for read operations
  read: {
    windowMs: 60 * 1000,
    maxRequests: 120,
  },
  // Strict limit for auth operations
  auth: {
    windowMs: 15 * 60 * 1000, // 15 minutes
    maxRequests: 10,          // 10 attempts per 15 min
  },
} as const;

export interface RateLimitResult {
  allowed: boolean;
  remaining: number;
  resetTime: number;
  retryAfter?: number;
}

/**
 * Check if request is within rate limit
 */
export function checkRateLimit(
  identifier: string,
  config: RateLimitConfig
): RateLimitResult {
  const now = Date.now();
  const key = `${config.keyPrefix || 'rl'}:${identifier}`;

  const entry = rateLimitStore.get(key);

  // Clean up expired entry
  if (entry && now > entry.resetTime) {
    rateLimitStore.delete(key);
  }

  // Get or create entry
  const current = rateLimitStore.get(key);
  if (!current) {
    const resetTime = now + config.windowMs;
    rateLimitStore.set(key, {
      count: 1,
      resetTime,
    });

    return {
      allowed: true,
      remaining: config.maxRequests - 1,
      resetTime,
    };
  }

  // Check limit
  if (current.count >= config.maxRequests) {
    const retryAfter = Math.ceil((current.resetTime - now) / 1000);

    logger.warn({
      identifier,
      key,
      count: current.count,
      resetTime: current.resetTime,
    }, "Rate limit exceeded");

    return {
      allowed: false,
      remaining: 0,
      resetTime: current.resetTime,
      retryAfter,
    };
  }

  // Increment counter
  current.count++;

  return {
    allowed: true,
    remaining: config.maxRequests - current.count,
    resetTime: current.resetTime,
  };
}

/**
 * Clean up expired entries periodically
 */
export function startRateLimitCleanup(): void {
  const CLEANUP_INTERVAL = 5 * 60 * 1000; // 5 minutes

  setInterval(() => {
    const now = Date.now();
    let cleaned = 0;

    for (const [key, entry] of rateLimitStore.entries()) {
      if (now > entry.resetTime) {
        rateLimitStore.delete(key);
        cleaned++;
      }
    }

    if (cleaned > 0) {
      logger.debug({ cleaned, remaining: rateLimitStore.size }, "Rate limit cleanup completed");
    }
  }, CLEANUP_INTERVAL);
}

/**
 * Get rate limit headers for response
 */
export function getRateLimitHeaders(result: RateLimitResult): Record<string, string> {
  return {
    "X-RateLimit-Limit": String(rateLimitConfigs.messageCreate.maxRequests),
    "X-RateLimit-Remaining": String(result.remaining),
    "X-RateLimit-Reset": String(Math.ceil(result.resetTime / 1000)),
    ...(result.retryAfter && { "Retry-After": String(result.retryAfter) }),
  };
}
