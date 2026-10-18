"use client";

import type { ReactNode } from "react";
import { useAuthStore } from "@multica/core/auth";
import { useT } from "@multica/views/i18n";
import { Button } from "@multica/ui/components/ui/button";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";

export function AuthRecoveryGate({ children }: { children: ReactNode }) {
  const status = useAuthStore((state) => state.status);
  const recoveryCause = useAuthStore((state) => state.recoveryCause);
  const retryAuthentication = useAuthStore(
    (state) => state.retryAuthentication,
  );
  const { t } = useT("auth");

  if (status !== "recovering") return <>{children}</>;

  const cause = recoveryCause ?? "network";
  const title =
    cause === "database"
      ? t(($) => $.desktop.recovery.database.title)
      : cause === "service"
        ? t(($) => $.desktop.recovery.service.title)
        : t(($) => $.desktop.recovery.network.title);
  const description =
    cause === "database"
      ? t(($) => $.desktop.recovery.database.description)
      : cause === "service"
        ? t(($) => $.desktop.recovery.service.description)
        : t(($) => $.desktop.recovery.network.description);

  return (
    <main className="flex h-svh items-center justify-center bg-background p-8">
      <div className="flex max-w-md flex-col items-center text-center">
        <MulticaIcon bordered size="lg" />
        <h1 className="mt-6 text-title font-semibold">
          {title}
        </h1>
        <p className="mt-2 text-body text-muted-foreground">
          {description}
        </p>
        <Button className="mt-6" onClick={retryAuthentication}>
          {t(($) => $.desktop.recovery.retry)}
        </Button>
      </div>
    </main>
  );
}
