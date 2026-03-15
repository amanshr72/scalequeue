import { Request, Response } from "express";
import { JobService } from "../services/jobService";
import { createJobSchema } from "../validators/jobValidator";
import { ZodError } from "zod";

const jobService = new JobService();

export class JobController {
  createJob = async (req: Request, res: Response): Promise<void> => {
    try {
      const input = createJobSchema.parse(req.body);
      const job = await jobService.createJob(input);

      res.status(202).json({
        success: true,
        data: job,
        message: "Job accepted and queued for processing",
      });
    } catch (error) {
      if (error instanceof ZodError) {
        res.status(400).json({
          success: false,
          error: "Validation failed",
          details: error.issues.map((e) => ({
            field: e.path.join("."),
            message: e.message,
          })),
        });
        return;
      }

      console.error("[JobController] createJob error:", error);
      res.status(500).json({
        success: false,
        error: "Internal server error",
      });
    }
  };

  getJobById = async (req: Request, res: Response): Promise<void> => {
    try {
      const { id } = req.params as { id: string };

      if (!id || id.length !== 36) {
        res.status(400).json({
          success: false,
          error: "Invalid job ID format",
        });
        return;
      }

      const job = await jobService.getJobById(id);

      if (!job) {
        res.status(404).json({ success: false, error: "Job not found" });
        return;
      }

      res.json({ success: true, data: job });
    } catch (error) {
      console.error("[JobController] getJobById error:", error);
      res.status(500).json({ success: false, error: "Internal server error" });
    }
  };

  listJobs = async (req: Request, res: Response): Promise<void> => {
    try {
      const {
        status,
        type,
        limit = "20",
        offset = "0",
      } = req.query as Record<string, string>;

      const parsedLimit = Math.min(parseInt(limit, 10) || 20, 100); // cap at 100
      const parsedOffset = parseInt(offset, 10) || 0;

      const result = await jobService.listJobs({
        status,
        type,
        limit: parsedLimit,
        offset: parsedOffset,
      });

      res.json({
        success: true,
        data: result.jobs,
        pagination: {
          total: result.total,
          limit: parsedLimit,
          offset: parsedOffset,
        },
      });
    } catch (error) {
      console.error("[JobController] listJobs error:", error);
      res.status(500).json({ success: false, error: "Internal server error" });
    }
  };

  listDeadJobs = async (req: Request, res: Response): Promise<void> => {
    try {
      const limit = Math.min(
        parseInt((req.query.limit as string) || "20"),
        100,
      );
      const offset = parseInt((req.query.offset as string) || "0");

      const result = await jobService.listDeadJobs(limit, offset);
      res.json({
        success: true,
        data: result.jobs,
        pagination: { total: result.total, limit, offset },
      });
    } catch (error) {
      console.error("[JobController] listDeadJobs error:", error);
      res.status(500).json({ success: false, error: "Internal server error" });
    }
  };

  requeueJob = async (req: Request, res: Response): Promise<void> => {
    try {
      const { id } = req.params as { id: string };

      if (!id || id.length !== 36) {
        res.status(400).json({
          success: false,
          error: "Invalid job ID format",
        });
        return;
      }

      const job = await jobService.requeueJob(id);
      res.json({
        success: true,
        data: job,
        message: "Job requeued successfully",
      });
    } catch (error: any) {
      if (error.message === "JOB_NOT_FOUND") {
        res.status(404).json({ success: false, error: "Job not found" });
        return;
      }
      if (error.message === "JOB_NOT_REQUEUEABLE") {
        res.status(400).json({
          success: false,
          error: "Only DEAD or FAILED jobs can be requeued",
        });
        return;
      }
      console.error("[JobController] requeueJob error:", error);
      res.status(500).json({ success: false, error: "Internal server error" });
    }
  };

  getJobLogs = async (req: Request, res: Response): Promise<void> => {
    try {
      const { id } = req.params as { id: string };

      if (!id || id.length !== 36) {
        res.status(400).json({
          success: false,
          error: "Invalid job ID format",
        });
        return;
      }

      const logs = await jobService.getJobLogs(id);
      if (!logs.length) {
        res
          .status(404)
          .json({ success: false, error: "No logs found for this job" });
        return;
      }
      res.json({ success: true, data: logs });
    } catch (error) {
      console.error("[JobController] getJobLogs error:", error);
      res.status(500).json({ success: false, error: "Internal server error" });
    }
  };
}
