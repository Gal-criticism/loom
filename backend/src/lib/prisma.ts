import { PrismaClient } from "@prisma/client";

const globalForPrisma = global as unknown as { prisma: PrismaClient };

export const prisma = globalForPrisma.prisma || new PrismaClient();

if (process.env.NODE_ENV !== "production") {
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
        console.log(`${model}.${operation} took ${duration.toFixed(2)}ms`);
        return result;
      },
      async findUnique({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        console.log(`${model}.${operation} took ${duration.toFixed(2)}ms`);
        return result;
      },
      async findFirst({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        console.log(`${model}.${operation} took ${duration.toFixed(2)}ms`);
        return result;
      },
      async create({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        console.log(`${model}.${operation} took ${duration.toFixed(2)}ms`);
        return result;
      },
      async update({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        console.log(`${model}.${operation} took ${duration.toFixed(2)}ms`);
        return result;
      },
      async delete({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        console.log(`${model}.${operation} took ${duration.toFixed(2)}ms`);
        return result;
      },
      async count({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        console.log(`${model}.${operation} took ${duration.toFixed(2)}ms`);
        return result;
      },
    },
  },
});
