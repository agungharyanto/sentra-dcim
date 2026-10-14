import { Route, Routes } from "react-router";

import { NotFoundPage } from "@/app/pages/StatusPages";

import { AuditPage } from "./AuditPage";

/** Audit Log (/audit, permission audit.read). */
export function AuditModule() {
  return (
    <Routes>
      <Route index element={<AuditPage />} />
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
