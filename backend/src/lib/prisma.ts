import { PrismaClient } from "@prisma/client";
import { config } from "./config";
import { logger } from "./logger";

const globalForPrisma = global as unknown as { prisma: PrismaClient };

export const prisma = globalForPrisma.prisma || new PrismaClient({
  datasources: {
    db: {
      url: config.database.url,
    },
  },
});

if (config.app.env !== "production") {
  globalForPrisma.prisma = prisma;
}

// Extended client with logging
export const db = prisma.$extends({
  query: {
    $allModels: {
      async findMany({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        logger.debug({ model, operation, duration: `${duration.toFixed(2)}ms` }, "Database query");
        return result;
      },
      async findUnique({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        logger.debug({ model, operation, duration: `${duration.toFixed(2)}ms` }, "Database query");
        return result;
      },
      async findFirst({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        logger.debug({ model, operation, duration: `${duration.toFixed(2)}ms` }, "Database query");
        return result;
      },
      async create({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        logger.debug({ model, operation, duration: `${duration.toFixed(2)}ms` }, "Database query");
        return result;
      },
      async update({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        logger.debug({ model, operation, duration: `${duration.toFixed(2)}ms` }, "Database query");
        return result;
      },
      async delete({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        logger.debug({ model, operation, duration: `${duration.toFixed(2)}ms` }, "Database query");
        return result;
      },
      async count({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        logger.debug({ model, operation, duration: `${duration.toFixed(2)}ms` }, "Database query");
        return result;
      },
    },
  },
});
