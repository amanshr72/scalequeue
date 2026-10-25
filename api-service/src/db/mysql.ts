import mysql from "mysql2/promise";
import { config } from "../config/index";

const pool = mysql.createPool({
  host: config.db.host,
  port: config.db.port,
  user: config.db.user,
  password: config.db.password,
  database: config.db.name,

  waitForConnections: true,
  connectionLimit: 10,
  queueLimit: 0,
  timezone: "+00:00",

  ssl: process.env.DB_SSL === "true" ? { rejectUnauthorized: true } : undefined,
});

// Test connection on startup
export async function testConnection(): Promise<void> {
  const conn = await pool.getConnection();
  console.log("[DB] MySQL connected successfully");
  conn.release();
}

export default pool;
