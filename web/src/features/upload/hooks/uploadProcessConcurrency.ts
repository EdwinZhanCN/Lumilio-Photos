export interface Semaphore {
  acquire: () => Promise<void>;
  release: () => void;
}

export const createSemaphore = (limit: number): Semaphore => {
  let available = Math.max(1, Math.floor(limit));
  const queue: Array<() => void> = [];

  return {
    async acquire() {
      if (available > 0) {
        available -= 1;
        return;
      }
      await new Promise<void>((resolve) => queue.push(resolve));
    },
    release() {
      const next = queue.shift();
      if (next) {
        next();
      } else {
        available += 1;
      }
    },
  };
};
