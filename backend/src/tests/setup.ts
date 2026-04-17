// Test setup file
import { beforeAll, afterAll } from "bun:test";
import { prisma } from "../lib/prisma";

// Test database URL must be set
const TEST_DATABASE_URL = process.env.DATABASE_URL;

if (!TEST_DATABASE_URL?.includes("test")) {
  throw new Error(
    "DATABASE_URL must contain 'test' to prevent accidental data loss in production"
  );
}

// Clean up database before tests
beforeAll(async () => {
  // Delete test data in correct order (respect foreign keys)
  await prisma.message.deleteMany({});
  await prisma.session.deleteMany({});
  await prisma.character.deleteMany({});
  await prisma.user.deleteMany({});
});

// Disconnect after tests
afterAll(async () => {
  await prisma.$disconnect();
});
