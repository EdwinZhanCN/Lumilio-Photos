import { waitForRepositoryScan } from "@/features/repositories";
import { uploadFile } from "@/lib/upload/uploadTransport";
import { waitForUploadJobs } from "@/lib/upload/uploadLifecycle";
import { AppWorkerClient } from "@/workers/workerClient";

interface ProductionSmokeAPI {
  hash: () => Promise<string>;
  uploadFailure: () => Promise<string>;
  uploadLifecycle: () => Promise<string[]>;
  scanLifecycle: () => Promise<string>;
}

declare global {
  interface Window {
    __lumilioProductionSmoke?: ProductionSmokeAPI;
  }
}

window.__lumilioProductionSmoke = {
  async hash() {
    const worker = new AppWorkerClient();
    try {
      let digest = "";
      await worker.generateHash([new File(["lumilio"], "smoke.jpg")], (result) => {
        digest = result.hash;
      });
      return digest;
    } finally {
      worker.terminateAllWorkers();
    }
  },

  async uploadFailure() {
    try {
      await uploadFile(new File(["photo"], "photo.jpg"), "smoke-hash");
      return "unexpected success";
    } catch (error) {
      return error instanceof Error ? error.message : String(error);
    }
  },

  async uploadLifecycle() {
    const states: string[] = [];
    await waitForUploadJobs([42], {
      intervalMs: 0,
      timeoutMs: 1_000,
      onUpdate: (job) => states.push(job.status || ""),
    });
    return states;
  },

  async scanLifecycle() {
    const scan = await waitForRepositoryScan("repo-1", Date.now(), {
      intervalMs: 0,
      timeoutMs: 1_000,
    });
    return scan.status || "";
  },
};
