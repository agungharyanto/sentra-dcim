import { Route, Routes } from "react-router";

import { NotFoundPage } from "@/app/pages/StatusPages";

import { UserDetailPage } from "./UserDetailPage";
import { UsersPage } from "./UsersPage";

/** Users & Roles (/users, permission users.read): list + roles tab, and /users/:id. */
export function UsersModule() {
  return (
    <Routes>
      <Route index element={<UsersPage />} />
      <Route path=":id" element={<UserDetailPage />} />
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
