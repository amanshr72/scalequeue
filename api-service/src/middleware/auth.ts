import { Request, Response, NextFunction } from "express";
import pool from "../db/mysql";

declare global {
  namespace Express {
    interface Request {
      clientId?: string;
      clientName?: string;
    }
  }
}

export async function apiKeyAuth(
  req: Request,
  res: Response,
  next: NextFunction,
): Promise<void> {
  const apiKey = req.headers["x-api-key"] as string;
  if (!apiKey) {
    res.status(401).json({
      success: false,
      error: "Missing API key. Pass it as x-api-key header.",
    });
    return;
  }

  try {
    const [rows] = await pool.execute<any[]>(
      `SELECT id, name, is_active 
       FROM api_clients 
       WHERE api_key = ? LIMIT 1`,
      [apiKey],
    );

    if (rows.length === 0) {
      res.status(401).json({ success: false, error: "Invalid API key" });
      return;
    }

    const client = rows[0];

    if (!client.is_active) {
      res.status(403).json({ success: false, error: "API key is disabled" });
      return;
    }

    // Attach client info to request for downstream use
    req.clientId = client.id;
    req.clientName = client.name;

    next();
  } catch (error) {
    console.error("[Auth] Database error during auth:", error);
    res.status(500).json({ success: false, error: "Auth check failed" });
  }
}
