/** A pending task is cancellable, but only a running task proves execution. */
export function chatTaskIsRunning(status: string | undefined): boolean {
  return status === "running";
}
