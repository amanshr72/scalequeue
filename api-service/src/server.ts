import express from "express";
import cors from "cors";
import helmet from "helmet";
import { jobRoutes } from "./routes/jobs";
import { config } from "./config";
import pool, { testConnection } from "./db/mysql";
import { queueService } from "./services/queueService";
import { apiKeyAuth } from "./middleware/auth";

const app = express();

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: "1mb" }));

app.get("/health", async (req, resp) => {
  const dbOk = await pool
    .query("SELECT 1")
    .then(() => true)
    .catch(() => false);
  const redisOk = await queueService.ping();
  const status = dbOk && redisOk ? "ok" : "degraded";
  resp.status(status === "ok" ? 200 : 503).json({
    status: status,
    services: {
      database: dbOk ? "up" : "down",
      redis: redisOk ? "up" : "down",
    },
  });
});

app.use("/api/v1/jobs", jobRoutes);
// app.use("/api/v1/jobs", apiKeyAuth, jobRoutes);

app.use((req, resp) => {
  resp.status(404).json({ error: "Route not found" });
});

async function bootstrap() {
  try {
    await testConnection();
    app.listen(config.port, () => {
      console.log(
        `[Server] Running on port ${config.port} in ${config.nodeEnv} enviroment`,
      );
    });
  } catch (error) {
    console.log("[Server] Failed to start: ", error);
    process.exit(1);
  }
}

bootstrap();
