import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, X } from "lucide-react";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { toast } from "@/components/ui/toast";
import { FormMessage } from "@/features/auth/FormMessage";
import { usePermissions } from "@/features/auth/permissions";
import { facilityKey, fetchSites, PERM_FACILITY_READ } from "@/features/facility/api";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { setSiteRoles, SYSTEM_ROLES, type User, usersKey } from "./api";
import { useUserText } from "./text";

const selectClass =
  "h-9 w-full rounded-md border border-input bg-background px-3 text-base shadow-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring md:text-sm";

/** Role assignments limited to one site (v0.4.1, D-084). Admin is global only and not offered. */
export function SiteRolesCard({ user, canManage }: { user: User; canManage: boolean }) {
  const { t } = useTranslation(["users", "errors"]);
  const id = useId();
  const qc = useQueryClient();
  const perms = usePermissions();
  const { roleLabel } = useUserText();
  const initial = user.site_roles ?? [];
  const [rows, setRows] = useState(initial.map((r) => ({ role: r.role, site_id: r.site_id })));
  const [role, setRole] = useState("technician");
  const [site, setSite] = useState("");
  const [error, setError] = useState<string | null>(null);
  const sites = useQuery({
    queryKey: [...facilityKey, "sites"],
    queryFn: ({ signal }) => fetchSites(signal),
    enabled: canManage && perms.has(PERM_FACILITY_READ),
  });
  const siteLabel = (sid: string) => {
    const known = initial.find((r) => r.site_id === sid);
    const s = sites.data?.items.find((x) => x.id === sid);
    return s ? `${s.code} · ${s.name}` : known ? `${known.site_code} · ${known.site_name}` : sid;
  };
  const save = useMutation({
    mutationFn: () => setSiteRoles(user.id, rows),
    onSuccess: (u) => {
      setError(null);
      qc.setQueryData([...usersKey, "detail", user.id], u);
      void qc.invalidateQueries({ queryKey: [...usersKey, "list"] });
      toast.success(t("siteRoles.saved"));
    },
    onError: (err) => setError(t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`)),
  });
  const add = () => {
    if (!site) return;
    if (rows.some((r) => r.role === role && r.site_id === site)) {
      setError(t("siteRoles.duplicate"));
      return;
    }
    setError(null);
    setRows([...rows, { role, site_id: site }]);
  };
  const dirty = JSON.stringify(rows) !== JSON.stringify(initial.map((r) => ({ role: r.role, site_id: r.site_id })));
  if (!canManage && initial.length === 0) return null;
  return (
    <Card data-testid="site-roles">
      <CardHeader>
        <CardTitle>{t("siteRoles.title")}</CardTitle>
        <CardDescription>{t("siteRoles.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error && <FormMessage tone="error">{error}</FormMessage>}
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("siteRoles.empty")}</p>
        ) : (
          <ul className="divide-y rounded-lg border text-sm">
            {rows.map((r) => (
              <li key={`${r.role}@${r.site_id}`} className="flex items-center gap-2 px-3 py-2" data-testid="site-role">
                <span className="font-medium">{roleLabel(r.role)}</span>
                <span className="min-w-0 flex-1 truncate text-muted-foreground">{siteLabel(r.site_id)}</span>
                {initial.find((x) => x.site_id === r.site_id)?.site_archived && (
                  <span className="text-xs text-muted-foreground">{t("siteRoles.archived")}</span>
                )}
                {canManage && (
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={t("siteRoles.remove", { role: roleLabel(r.role), site: siteLabel(r.site_id) })}
                    onClick={() => setRows(rows.filter((x) => x !== r))}
                  >
                    <X aria-hidden />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
        {canManage && (
          <>
            <div className="grid gap-2 sm:grid-cols-[1fr_2fr_auto] sm:items-end">
              <div className="space-y-1.5">
                <label htmlFor={`${id}-role`} className="text-sm font-medium">
                  {t("siteRoles.role")}
                </label>
                <select id={`${id}-role`} className={selectClass} value={role} onChange={(e) => setRole(e.target.value)}>
                  {SYSTEM_ROLES.filter((r) => r !== "admin").map((r) => (
                    <option key={r} value={r}>
                      {roleLabel(r)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="space-y-1.5">
                <label htmlFor={`${id}-site`} className="text-sm font-medium">
                  {t("siteRoles.site")}
                </label>
                <select id={`${id}-site`} className={selectClass} value={site} onChange={(e) => setSite(e.target.value)} data-testid="site-role-site">
                  <option value="" />
                  {(sites.data?.items ?? []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {`${s.code} · ${s.name}`}
                    </option>
                  ))}
                </select>
              </div>
              <Button variant="outline" onClick={add} disabled={!site} data-testid="site-role-add">
                <Plus aria-hidden />
                {t("siteRoles.add")}
              </Button>
            </div>
            <Button disabled={!dirty || save.isPending} onClick={() => save.mutate()} data-testid="save-site-roles">
              {save.isPending ? t("detail.saving") : t("siteRoles.save")}
            </Button>
          </>
        )}
      </CardContent>
    </Card>
  );
}
