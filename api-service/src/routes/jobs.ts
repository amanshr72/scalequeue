import { Router } from "express";
import { JobController } from '../controllers/jobController';

const router = Router();
const controller = new JobController();

router.post("/", controller.createJob);
router.get("/dead", controller.listDeadJobs);
router.post("/:id/requeue", controller.requeueJob);
router.get("/:id/logs", controller.getJobLogs);
router.get("/:id", controller.getJobById);
router.get("/", controller.listJobs);

export const jobRoutes = router;
