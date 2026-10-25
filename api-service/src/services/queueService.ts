import Redis from "ioredis";
import { config } from "../config";

class QueueService {
  private client: Redis;
  private readonly STREAM_NAME = "jobs:queue";

  constructor() {
    this.client = new Redis(config.redis.url, {
      maxRetriesPerRequest: 3,
      lazyConnect: true,
    });

    this.client.on("connect", () => {
      console.log("[Redis] Connected sucessfully");
    });

    this.client.on("error", (err) => {
      console.error(`[Redis] Connection error: ${err.message}`);
    });
  }

  async close(): Promise<void> { this.client.disconnect(); }

  async ping(): Promise<boolean> {
    // try {
    //   const res = await this.client.ping();
    //   return res === "PONG";
    // } catch (err) {
    //   console.error("[Redis] Ping failed: ", (err as Error).message);
    //   return false;
    // }
    return this.client
      .ping()
      .then((res) => res === "PONG")
      .catch(() => false);
  }
}

export const queueService = new QueueService();
