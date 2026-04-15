import { Pool } from "pg";

export const pool = new Pool({
  connectionString: process.env.DATABASE_URL || "postgresql://loom:loom_dev@localhost:5432/loom",
});

export const db = {
  query: async (text: string, params?: any[]) => {
    const result = await pool.query(text, params);
    return result;
  },
};
